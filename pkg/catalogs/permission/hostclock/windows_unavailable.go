//go:build !windows

package hostclock

import (
	"context"
)

func readWindowsEvidence(context.Context) (windowsEvidence, error) {
	return windowsEvidence{}, invalidClock("Windows time-service observation requires Windows")
}
