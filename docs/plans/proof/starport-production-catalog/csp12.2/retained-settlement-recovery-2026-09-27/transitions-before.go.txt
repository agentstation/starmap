package reservation

import (
	"context"
	"errors"
	"maps"

	"github.com/agentstation/starport/internal/limits"
	"github.com/agentstation/starport/internal/storage"
)

// Begin consumes dispatch permission once. Only a successful return permits a
// provider call. An ambiguous storage response must not trigger dispatch.
func (r *Repository) Begin(ctx context.Context, id string) error {
	for range maxConflicts {
		record, old, err := r.readRecord(ctx, id)
		if err != nil {
			return err
		}
		if record.State != Reserved {
			return ErrAlreadyDispatched
		}
		record.State = Dispatched
		mutation, err := encodeMutation(storageKey("attempt", id), old, record)
		if err != nil {
			return err
		}
		err = r.store.CompareAndSwapInWindow(ctx, []storage.CompareAndSwapMutation{mutation}, bindingWindow(record.Bindings))
		if !errors.Is(err, storage.ErrConflict) {
			return err
		}
	}
	return ErrUnavailable
}

// MarkUncertain records why capacity remains deducted. It does not expire or refund it.
func (r *Repository) MarkUncertain(ctx context.Context, id, reason string) error {
	if !validID(reason) {
		return ErrInvalid
	}
	for range maxConflicts {
		record, old, err := r.readRecord(ctx, id)
		if err != nil {
			return err
		}
		if record.State == Uncertain {
			return nil
		}
		if record.State != Dispatched {
			return ErrTransition
		}
		record.State, record.Reason = Uncertain, reason
		mutation, err := encodeMutation(storageKey("attempt", id), old, record)
		if err != nil {
			return err
		}
		err = r.store.CompareAndSwapInWindow(ctx, []storage.CompareAndSwapMutation{mutation}, storage.TimeWindow{})
		if !errors.Is(err, storage.ErrConflict) {
			return err
		}
	}
	return ErrUnavailable
}

// CancelBeforeDispatch releases only capacity whose dispatch permit remains unused.
func (r *Repository) CancelBeforeDispatch(ctx context.Context, id string) error {
	return r.finish(ctx, id, nil)
}

// Reconcile applies reliable usage once under the original prices and windows.
// Different evidence for an already settled identity requires operator repair.
func (r *Repository) Reconcile(ctx context.Context, id string, evidence Evidence) error {
	if !validID(evidence.ID) || evidence.Tokens < 0 {
		return ErrInvalid
	}
	return r.finish(ctx, id, &evidence)
}

func (r *Repository) finish(ctx context.Context, id string, evidence *Evidence) error {
	for range maxConflicts {
		if err := ctx.Err(); err != nil {
			return err
		}
		record, old, err := r.readRecord(ctx, id)
		if err != nil {
			return err
		}
		var amount int64
		var overflow bool
		if evidence == nil {
			if record.State == Canceled {
				return nil
			}
			if record.State != Reserved {
				return ErrTransition
			}
			record.State, record.NanoUSD = Canceled, record.Attempt.money(0)
		} else {
			if record.Pending != nil && !sameEvidence(record.Pending, evidence) {
				return ErrIdentityConflict
			}
			if record.Unresolved != nil && !sameEvidence(record.Unresolved, evidence) {
				return ErrIdentityConflict
			}
			if record.State == Settled {
				if sameEvidence(record.Evidence, evidence) {
					return nil
				}
				return ErrIdentityConflict
			}
			if record.State != Dispatched && record.State != Uncertain {
				return ErrTransition
			}
			amount, err = record.Attempt.amount(evidence.Quantities)
			overflow = errors.Is(err, ErrOverflow)
			if err != nil && !overflow {
				return err
			}
			if overflow {
				if sameEvidence(record.Unresolved, evidence) {
					return ErrOverflow
				}
				record.State, record.Unresolved, record.Reason = Uncertain, evidence, "valuation_overflow"
				record.Pending = nil
			} else {
				record.State, record.Evidence, record.NanoUSD, record.Reason = Settled, evidence, record.Attempt.money(amount), ""
				record.Unresolved = nil
				record.Pending = nil
			}
		}
		mutations := make([]storage.CompareAndSwapMutation, 0, len(record.Bindings)+1)
		for _, binding := range record.Bindings {
			state, previous, err := r.readWindow(ctx, binding.Rule.Meter, binding.Window)
			if err != nil {
				return err
			}
			if state.Reserved < binding.Amount {
				return ErrUnavailable
			}
			if overflow {
				state.Overflow = true
			} else {
				state.Reserved -= binding.Amount
				actual := amount
				if evidence != nil && binding.Rule.Meter.Dimension == limits.DimensionTokens {
					actual = evidence.Tokens
				}
				consume(state, actual)
			}
			mutation, err := encodeMutation(meterKey(state.Meter, state.Window), previous, state)
			if err != nil {
				return err
			}
			mutations = append(mutations, mutation)
		}
		mutation, err := encodeMutation(storageKey("attempt", id), old, record)
		if err != nil {
			return err
		}
		mutations = append(mutations, mutation)
		err = r.store.CompareAndSwapInWindow(ctx, mutations, storage.TimeWindow{})
		if err == nil && overflow {
			return ErrOverflow
		}
		if !errors.Is(err, storage.ErrConflict) {
			return err
		}
	}
	return ErrUnavailable
}

// JSON normalizes an absent quantity map to an empty object. Both represent no
// monetary components for token-only evidence and must preserve exact retries.
func sameEvidence(first, second *Evidence) bool {
	if first == nil || second == nil {
		return first == second
	}
	return first.ID == second.ID && first.Tokens == second.Tokens && maps.Equal(first.Quantities, second.Quantities)
}
