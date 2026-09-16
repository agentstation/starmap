package administration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

func (m *Manager) recover(ctx context.Context) error {
	if err := m.writer.operations.RecoverPublications(ctx); err != nil {
		return err
	}
	if err := m.checkIdentityHistory(); err != nil {
		return err
	}
	for _, receipt := range m.audit.receipts {
		if err := m.checkReceipt(receipt); err != nil {
			return err
		}
		if receipt.State == Accepted {
			state := Interrupted
			if receipt.AfterDigest != "" {
				switch contentDigest(m.encoded) {
				case receipt.AfterDigest:
					state = Succeeded
				case receipt.BeforeDigest:
					state = Failed
				}
			}
			if _, err := m.finish(ctx, receipt, state); err != nil {
				return err
			}
		} else if err := m.persistReceipt(ctx, receipt); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) checkIdentityHistory() error {
	latest := m.audit.receipts[m.audit.latestIdentity]
	if latest.ID == "" {
		return invalidAudit()
	}
	digest := contentDigest(m.encoded)
	switch latest.State {
	case Succeeded:
		if latest.Revision != m.record.Revision || latest.AfterDigest != digest {
			return invalidAudit()
		}
	case Failed:
		if latest.Revision != m.record.Revision+1 || latest.BeforeDigest != digest {
			return invalidAudit()
		}
	case Accepted:
		if latest.Revision == m.record.Revision && latest.AfterDigest == digest {
			return nil
		}
		if latest.Revision == m.record.Revision+1 && latest.BeforeDigest == digest {
			return nil
		}
		return invalidAudit()
	default:
		return invalidAudit()
	}
	return nil
}

func (m *Manager) checkReceipt(expected Receipt) error {
	data, err := m.writer.operations.ReadFile(receiptName(expected.ID), maximumAuditEventBytes)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return invalidAudit()
	}
	canonical, err := json.Marshal(receipt)
	if err != nil || !bytes.Equal(data, canonical) || !receipt.valid(m.audience) {
		return invalidAudit()
	}
	if receipt == expected {
		return nil
	}
	if receipt.State != Accepted {
		return invalidAudit()
	}
	comparison := expected
	comparison.State = receipt.State
	comparison.CompletedAt = receipt.CompletedAt
	if comparison != receipt {
		return invalidAudit()
	}
	return nil
}

// StartOperation records an administrator's operation intent before the caller starts its effects.
func (m *Manager) StartOperation(ctx context.Context, actor Principal, action Action, subject string) (Receipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.authorize(actor); err != nil {
		return Receipt{}, err
	}
	if (action != RefreshCatalog && action != CancelOperation) || !validIdentity(subject) {
		return Receipt{}, &errors.ValidationError{Field: "server.operation", Message: "requires a supported action and a bounded subject"}
	}
	return m.begin(ctx, actor.id, action, subject, "", "", m.record.Revision)
}

// Complete persists the operation outcome before its caller reports success.
// A failed effect can have partial results. Recovery never executes the effect again.
func (m *Manager) Complete(ctx context.Context, id string, succeeded bool) (Receipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.writable(ctx); err != nil {
		return Receipt{}, err
	}
	receipt, exists := m.audit.receipts[id]
	if !exists {
		return Receipt{}, &errors.NotFoundError{Resource: "administrative operation", ID: id}
	}
	state := Failed
	if succeeded {
		state = Succeeded
	}
	if receipt.State == state {
		return receipt, nil
	}
	if receipt.State != Accepted || receipt.AfterDigest != "" {
		return Receipt{}, &errors.ConflictError{Resource: "administrative operation", Message: "operation cannot accept this outcome"}
	}
	return m.finish(ctx, receipt, state)
}

// Operation returns a retained durable operation receipt for authorized diagnostics.
func (m *Manager) Operation(actor Principal, id string) (Receipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.authorize(actor); err != nil {
		return Receipt{}, err
	}
	if m.audit != nil {
		if receipt, exists := m.audit.receipts[id]; exists {
			return receipt, nil
		}
	}
	return Receipt{}, &errors.NotFoundError{Resource: "administrative operation", ID: id}
}
