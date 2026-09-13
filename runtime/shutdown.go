package runtime

import (
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// joinRuntimeShutdown bounds the caller's wait without canceling resource cleanup.
// The cleanup retains directory ownership until every owned worker stops.
func joinRuntimeShutdown(cleanup func() error) error {
	joined := make(chan error, 1)
	go func() { joined <- cleanup() }()
	timer := time.NewTimer(closeJoinTimeout)
	defer timer.Stop()
	select {
	case err := <-joined:
		return err
	case <-timer.C:
		return &errors.TimeoutError{Operation: "close starmap runtime", Duration: closeJoinTimeout.String()}
	}
}
