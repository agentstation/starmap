package administration

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"time"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	auditFile              = "events.ndjson"
	maximumAuditBytes      = 64 << 20
	maximumAuditEventBytes = 16 << 10
	maximumOperations      = 16384
)

type auditEvent struct {
	SchemaVersion int       `json:"schema_version"`
	Sequence      uint64    `json:"sequence"`
	RecordedAt    time.Time `json:"recorded_at"`
	Previous      string    `json:"previous"`
	Receipt       Receipt   `json:"receipt"`
}

type auditLog struct {
	latestIdentity string
	directory      *privatefiles.Directory
	entry          privatefiles.RecordReceipt
	sequence       uint64
	digest         string
	audience       string
	receipts       map[string]Receipt
	// beforeAppend injects a storage failure before any write in package tests.
	beforeAppend func() error
}

func openAudit(ctx context.Context, directory *privatefiles.Directory, audience string, create bool) (*auditLog, error) {
	data, err := directory.ReadFile(auditFile, maximumAuditBytes)
	if os.IsNotExist(err) && create {
		err = directory.CompareAndPublishFileContext(ctx, auditFile, nil, []byte{}, ".audit-")
		data = []byte{}
	}
	if err != nil {
		return nil, err
	}
	log := &auditLog{directory: directory, audience: audience, receipts: make(map[string]Receipt)}
	if err := log.replay(data); err != nil {
		return nil, err
	}
	root, err := directory.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	log.entry, err = privatefiles.CaptureRecord(root, auditFile, maximumAuditBytes)
	if err != nil {
		return nil, err
	}
	if log.entry.Digest != contentDigest(data) {
		return nil, administrationChanged()
	}
	return log, nil
}

func (l *auditLog) replay(data []byte) error {
	if len(data) > 0 && data[len(data)-1] != '\n' {
		return invalidAudit()
	}
	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		if len(line) > maximumAuditEventBytes {
			return invalidAudit()
		}
		var event auditEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return invalidAudit()
		}
		canonical, err := json.Marshal(event)
		if err != nil || !bytes.Equal(line, canonical) || event.SchemaVersion != identitySchemaVersion || event.RecordedAt.IsZero() ||
			event.Sequence != l.sequence+1 || event.Previous != l.digest {
			return invalidAudit()
		}
		if err := l.validateTransition(event.Receipt); err != nil {
			return err
		}
		l.receipts[event.Receipt.ID] = event.Receipt
		if event.Receipt.AfterDigest != "" {
			l.latestIdentity = event.Receipt.ID
		}
		l.sequence = event.Sequence
		l.digest = contentDigest(line)
	}
	return nil
}

func (l *auditLog) validateTransition(next Receipt) error {
	if !next.valid(l.audience) {
		return invalidAudit()
	}
	previous, exists := l.receipts[next.ID]
	if !exists {
		if next.State != Accepted || len(l.receipts) >= maximumOperations {
			return invalidAudit()
		}
		return nil
	}
	if previous.State != Accepted || next.State == Accepted {
		return invalidAudit()
	}
	comparison := next
	comparison.State = previous.State
	comparison.CompletedAt = previous.CompletedAt
	if comparison != previous {
		return invalidAudit()
	}
	return nil
}

func (l *auditLog) append(ctx context.Context, receipt Receipt, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := l.validateTransition(receipt); err != nil {
		return err
	}
	event := auditEvent{SchemaVersion: identitySchemaVersion, Sequence: l.sequence + 1, RecordedAt: now.UTC(), Previous: l.digest, Receipt: receipt}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(data) > maximumAuditEventBytes || l.entry.Size+int64(len(data)+1) > maximumAuditBytes {
		return auditCapacity()
	}
	if l.beforeAppend != nil {
		if err := l.beforeAppend(); err != nil {
			return err
		}
	}
	if err := l.appendBytes(ctx, append(data, '\n')); err != nil {
		return err
	}
	l.sequence = event.Sequence
	l.digest = contentDigest(data)
	l.receipts[receipt.ID] = receipt
	if receipt.AfterDigest != "" {
		l.latestIdentity = receipt.ID
	}
	return nil
}

func (l *auditLog) appendBytes(ctx context.Context, data []byte) (resultErr error) {
	root, err := l.directory.Open()
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	if err := privatefiles.CheckRecord(root, auditFile, l.entry); err != nil {
		return err
	}
	previous, err := privatefiles.ReadFile(root, auditFile, maximumAuditBytes)
	if err != nil {
		return err
	}
	if contentDigest(previous) != l.entry.Digest {
		return administrationChanged()
	}
	expected := contentDigest(append(previous, data...))
	before, err := root.Lstat(auditFile)
	if err != nil {
		return err
	}
	file, err := root.OpenFile(auditFile, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(before, opened) {
		return administrationChanged()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	written, err := file.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := filepublish.SyncDirectory(root); err != nil {
		return err
	}
	after, err := privatefiles.CaptureRecord(root, auditFile, maximumAuditBytes)
	if err != nil {
		return err
	}
	if after.Entry != l.entry.Entry || after.Size != l.entry.Size+int64(len(data)) || after.Digest != expected {
		return administrationChanged()
	}
	verified, err := l.directory.Open()
	if err != nil {
		return err
	}
	if err := verified.Close(); err != nil {
		return err
	}
	l.entry = after
	return nil
}

func invalidAudit() error {
	return &errors.ValidationError{Field: "server.audit", Message: "requires a complete ordered audit history with valid operation transitions"}
}

func auditCapacity() error {
	return &errors.ConfigError{Component: "server audit", Message: "retained history reached its capacity; new mutations are unavailable"}
}
