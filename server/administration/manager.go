package administration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	stderrors "errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// Config identifies the private state root and the catalog authority audience.
type Config struct {
	StateDirectory string
	Audience       string
}

// Status reports administration readiness without credential material.
type Status struct {
	MutationReady      bool   `json:"mutation_ready"`
	Reason             string `json:"reason,omitempty"`
	Revision           uint64 `json:"revision"`
	RetainedOperations int    `json:"retained_operations"`
	OperationLimit     int    `json:"operation_limit"`
	AuditBytes         int64  `json:"audit_bytes"`
	AuditByteLimit     int64  `json:"audit_byte_limit"`
}

// Manager owns identities, audit events, and operation receipts for one server.
// Authentication reads an immutable snapshot. Administrative writes use one process lease.
type Manager struct {
	changed  chan struct{}
	mu       sync.Mutex
	writer   *stateWriter
	audit    *auditLog
	snapshot atomic.Pointer[identitySnapshot]
	record   identitiesRecord
	encoded  []byte
	audience string
	clock    func() time.Time
	degraded bool
	closed   bool
}

// Initialize creates the first administrator through an explicit local operation.
// It refuses existing identities and returns the credential only after durable completion.
func Initialize(ctx context.Context, cfg Config, administratorID string) (_ *Manager, token string, resultErr error) {
	if !validAudience(cfg.Audience) || !validIdentity(administratorID) {
		return nil, "", invalidIdentityRecord()
	}
	writer, err := openStateWriter(ctx, cfg.StateDirectory, true)
	if err != nil {
		return nil, "", err
	}
	manager := &Manager{writer: writer, audience: cfg.Audience, clock: time.Now, changed: make(chan struct{})}
	defer func() {
		if resultErr != nil {
			_ = writer.close()
		}
	}()
	if _, err := writer.directory.ReadFile(identitiesFile, maximumIdentityRecordBytes); !os.IsNotExist(err) {
		if err != nil {
			return nil, "", err
		}
		return nil, "", &errors.ConflictError{Resource: "server identities", Message: "administrator initialization already exists"}
	}
	manager.audit, err = openAudit(ctx, writer.audit, cfg.Audience, true)
	if err != nil {
		return nil, "", err
	}
	if len(manager.audit.receipts) != 0 {
		return nil, "", &errors.ConflictError{Resource: "server bootstrap", Message: "an incomplete initialization requires local recovery"}
	}
	now := manager.clock()
	token, key, err := newCredential(Administrator, now)
	if err != nil {
		return nil, "", err
	}
	record := identitiesRecord{SchemaVersion: identitySchemaVersion, Audience: cfg.Audience, Revision: 1,
		Identities: []identityRecord{{ID: administratorID, Role: Administrator, Keys: []credentialRecord{key}}}}
	if err := manager.changeIdentities(ctx, administratorID, Bootstrap, administratorID, record); err != nil {
		return nil, "", err
	}
	return manager, token, nil
}

// Open restores private administration state without creating an administrator.
// Audit failures leave authenticated diagnostics available and reject new mutations.
func Open(ctx context.Context, cfg Config) (_ *Manager, resultErr error) {
	if !validAudience(cfg.Audience) {
		return nil, invalidIdentityRecord()
	}
	writer, err := openStateWriter(ctx, cfg.StateDirectory, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			_ = writer.close()
		}
	}()
	if err := writer.directory.RecoverPublications(ctx); err != nil {
		return nil, err
	}
	data, err := writer.directory.ReadFile(identitiesFile, maximumIdentityRecordBytes)
	if err != nil {
		return nil, err
	}
	record, snapshot, err := decodeIdentities(data, cfg.Audience)
	if err != nil {
		return nil, err
	}
	manager := &Manager{writer: writer, audience: cfg.Audience, record: record, encoded: data, clock: time.Now, changed: make(chan struct{})}
	manager.snapshot.Store(snapshot)
	if writer.storageErr != nil {
		manager.degraded = true
		return manager, nil
	}
	manager.audit, err = openAudit(ctx, writer.audit, cfg.Audience, false)
	if err != nil {
		manager.degraded = true
		return manager, nil
	}
	if err := manager.recover(ctx); err != nil {
		manager.degraded = true
	}
	return manager, nil
}

// Authenticate accepts an active credential for the configured audience without filesystem I/O.
func (m *Manager) Authenticate(token, audience string) (Principal, bool) {
	return m.snapshot.Load().authenticate(token, audience, m.clock())
}

// Status returns the current administration readiness and identity revision.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := Status{MutationReady: !m.degraded && !m.closed, Revision: m.record.Revision,
		OperationLimit: maximumOperations, AuditByteLimit: maximumAuditBytes}
	if m.audit != nil {
		status.RetainedOperations = len(m.audit.receipts)
		status.AuditBytes = m.audit.entry.Size
		if m.capacityReached() {
			status.MutationReady = false
			status.Reason = "retained_history_capacity"
		}
	}
	if m.degraded {
		status.Reason = "audit_or_state_unavailable"
	}
	if m.closed {
		status.Reason = "closed"
	}
	return status
}

// Close releases writer ownership and stops credential authentication.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	close(m.changed)
	m.snapshot.Store(nil)
	return m.writer.close()
}

func (m *Manager) authorize(actor Principal) error {
	snapshot := m.snapshot.Load()
	if snapshot == nil || actor.audience != m.audience {
		return administratorRequired()
	}
	current, accepted := snapshot.authenticateDigest(actor.key, m.clock())
	if !accepted || current != actor || current.role != Administrator {
		return administratorRequired()
	}
	return nil
}

func (m *Manager) writable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.closed || m.degraded || m.audit == nil {
		return &errors.ConfigError{Component: "server administration", Message: "mutations are unavailable; inspect audit and state recovery"}
	}
	if err := m.writer.check(); err != nil {
		m.degraded = true
		return err
	}
	return nil
}

func (m *Manager) persistReceipt(ctx context.Context, receipt Receipt) error {
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	return m.writer.operations.PublishFileContext(ctx, receiptName(receipt.ID), data, ".operation-")
}

func (m *Manager) begin(ctx context.Context, actor string, action Action, subject, before, after string, revision uint64) (Receipt, error) {
	if err := m.writable(ctx); err != nil {
		return Receipt{}, err
	}
	if m.capacityReached() {
		return Receipt{}, auditCapacity()
	}
	receipt := Receipt{ID: rand.Text(), Audience: m.audience, Actor: actor, Action: action, Subject: subject, State: Accepted,
		AcceptedAt: m.clock().UTC(), Revision: revision, BeforeDigest: before, AfterDigest: after}
	if err := m.audit.append(ctx, receipt, m.clock()); err != nil {
		m.degraded = true
		return Receipt{}, err
	}
	if err := m.persistReceipt(ctx, receipt); err != nil {
		m.degraded = true
		return Receipt{}, err
	}
	return receipt, nil
}

func (m *Manager) capacityReached() bool {
	return len(m.audit.receipts) >= maximumOperations || m.audit.entry.Size > maximumAuditBytes-2*maximumAuditEventBytes
}

func (m *Manager) finish(ctx context.Context, receipt Receipt, state State) (Receipt, error) {
	if err := m.writable(ctx); err != nil {
		return Receipt{}, err
	}
	receipt.State = state
	receipt.CompletedAt = m.clock().UTC()
	if receipt.CompletedAt.Before(receipt.AcceptedAt) {
		receipt.CompletedAt = receipt.AcceptedAt
	}
	if err := m.audit.append(ctx, receipt, m.clock()); err != nil {
		m.degraded = true
		return Receipt{}, err
	}
	if err := m.persistReceipt(ctx, receipt); err != nil {
		m.degraded = true
		return Receipt{}, err
	}
	return receipt, nil
}

func (m *Manager) changeIdentities(ctx context.Context, actor string, action Action, subject string, next identitiesRecord) error {
	snapshot, err := buildIdentitySnapshot(next, m.audience)
	if err != nil {
		return err
	}
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	before := ""
	if m.encoded != nil {
		before = contentDigest(m.encoded)
	}
	receipt, err := m.begin(ctx, actor, action, subject, before, contentDigest(data), next.Revision)
	if err != nil {
		return err
	}
	err = m.writer.directory.CompareAndPublishFileContext(ctx, identitiesFile, m.encoded, data, ".identities-")
	if err != nil {
		// Restrictive changes can already be visible after an uncertain publication.
		// Stop authentication until restart validates the durable record.
		var published *errors.PublicationError
		if stderrors.As(err, &published) {
			m.snapshot.Store(nil)
			m.notifyCredentials()
		}
		m.degraded = true
		return err
	}
	m.record, m.encoded = next, data
	m.snapshot.Store(snapshot)
	m.notifyCredentials()
	_, err = m.finish(ctx, receipt, Succeeded)
	return err
}

func administratorRequired() error {
	return &errors.AuthenticationError{Method: "administrator", Message: "an active administrator credential is required"}
}
