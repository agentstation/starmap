//go:build !windows

package w32timerpc

import (
	"context"
	"github.com/oiweiwei/go-msrpc/msrpc/w32t"
)

func observeLocalStatus(context.Context) (*w32t.StatusInfo, error) {
	return nil, invalidReply("local W32Time queries require Windows")
}

func newStatusSecurity() (statusSecurity, func(), error) {
	return nil, nil, invalidReply("local W32Time authentication requires Windows")
}
