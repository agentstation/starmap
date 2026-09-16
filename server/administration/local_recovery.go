package administration

import (
	"context"
	"os"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// RecoverAdministrator replaces a lost administrator credential through explicit local filesystem authority.
// It requires exclusive private state access and a valid audit history. Stop the server first.
func RecoverAdministrator(ctx context.Context, cfg Config, id string) (string, error) {
	if !validIdentity(id) || !validAudience(cfg.Audience) {
		return "", invalidIdentityRecord()
	}
	manager, err := Open(ctx, cfg)
	if os.IsNotExist(err) {
		return recoverInitialization(ctx, cfg, id)
	}
	if err != nil {
		return "", err
	}
	defer func() { _ = manager.Close() }()
	index := manager.findIdentity(id)
	if index < 0 || manager.record.Identities[index].Role != Administrator {
		return "", &errors.NotFoundError{Resource: "server administrator", ID: id}
	}
	if err := manager.writable(ctx); err != nil {
		return "", err
	}
	token, key, err := newCredential(Administrator, manager.clock())
	if err != nil {
		return "", err
	}
	next := manager.copyRecord()
	next.Identities[index].Revoked = false
	next.Identities[index].Keys = []credentialRecord{key}
	if err := manager.changeIdentities(ctx, "local-recovery", RotateIdentity, id, next); err != nil {
		return "", err
	}
	return token, nil
}

func recoverInitialization(ctx context.Context, cfg Config, id string) (string, error) {
	writer, err := openStateWriter(ctx, cfg.StateDirectory, false)
	if err != nil {
		return "", err
	}
	defer func() { _ = writer.close() }()
	if writer.storageErr != nil {
		return "", writer.storageErr
	}
	if err := writer.directory.RecoverPublications(ctx); err != nil {
		return "", err
	}
	if _, err := writer.directory.ReadFile(identitiesFile, maximumIdentityRecordBytes); !os.IsNotExist(err) {
		if err != nil {
			return "", err
		}
		return "", administrationChanged()
	}
	log, err := openAudit(ctx, writer.audit, cfg.Audience, false)
	if err != nil {
		return "", err
	}
	if len(log.receipts) == 0 {
		return "", invalidAudit()
	}
	manager := &Manager{writer: writer, audit: log, audience: cfg.Audience, clock: time.Now, changed: make(chan struct{})}
	for _, receipt := range log.receipts {
		if receipt.Action != Bootstrap || receipt.Subject != id || receipt.Revision != 1 || receipt.BeforeDigest != "" || receipt.State == Succeeded {
			return "", invalidAudit()
		}
		if err := manager.checkReceipt(receipt); err != nil {
			return "", err
		}
	}
	for _, receipt := range log.receipts {
		if receipt.State == Accepted {
			if _, err := manager.finish(ctx, receipt, Interrupted); err != nil {
				return "", err
			}
		}
	}
	token, key, err := newCredential(Administrator, manager.clock())
	if err != nil {
		return "", err
	}
	record := identitiesRecord{SchemaVersion: identitySchemaVersion, Audience: cfg.Audience, Revision: 1,
		Identities: []identityRecord{{ID: id, Role: Administrator, Keys: []credentialRecord{key}}}}
	if err := manager.changeIdentities(ctx, "local-recovery", Bootstrap, id, record); err != nil {
		return "", err
	}
	return token, nil
}

// Audience returns the immutable catalog namespace bound to this administration store.
func (m *Manager) Audience() string { return m.audience }
