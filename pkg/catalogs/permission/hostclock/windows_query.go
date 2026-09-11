//go:build windows

package hostclock

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock/internal/w32timerpc"
	"github.com/agentstation/starmap/pkg/errors"
)

func readWindowsEvidence(ctx context.Context) (windowsEvidence, error) {
	sample, err := w32timerpc.ObserveStatus(ctx)
	if err != nil {
		return windowsEvidence{}, errors.WrapResource("observe", "Windows time service", "", err)
	}
	return windowsEvidence{
		LeapIndicator: sample.LeapIndicator, Stratum: sample.Stratum,
		LastSyncTicks: sample.LastSyncTicks, TimeLastGoodSync: sample.TimeLastGoodSync,
		RootDispersion: sample.RootDispersion, RootDelay: sample.ToRootDelay,
		PhaseOffset: sample.ToSystemPhaseOffset, ClockPrecision: sample.ClockPrecision,
		Source: sample.Source, State: sample.LCState, Flags: sample.TSFlags, LastSyncResult: sample.LastSyncResult,
	}, nil
}
