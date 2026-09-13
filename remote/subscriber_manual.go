package remote

import (
	"context"

	"github.com/agentstation/starmap/pkg/errors"
)

// startManualRead owns a finite read through the subscriber's shutdown contract.
// Concurrent starts and reads return a conflict.
func (s *Subscriber) startManualRead(ctx context.Context) (context.Context, func(), error) {
	if ctx == nil {
		return nil, nil, &errors.ValidationError{Field: "remote.context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	readCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if s.state != stateIdle {
		actual := s.state.String()
		s.mu.Unlock()
		cancel()
		return nil, nil, &errors.ConflictError{
			Resource: "remote catalog subscriber", Expected: "idle", Actual: actual,
			Message: "manual reads require an idle subscriber",
		}
	}
	done := make(chan struct{})
	s.state = stateReading
	s.cancel = cancel
	s.done = done
	s.mu.Unlock()
	finish := func() {
		cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.state == stateReading {
			s.state = stateIdle
		}
		s.cancel = nil
		s.done = nil
		close(done)
	}
	return readCtx, finish, nil
}
