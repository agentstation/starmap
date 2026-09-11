package w32timerpc

import (
	"context"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/oiweiwei/go-msrpc/msrpc/w32t"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func observeLocalStatus(ctx context.Context) (*w32t.StatusInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, err
	}
	defer func() { _ = windows.CloseServiceHandle(manager) }()
	name, err := windows.UTF16PtrFromString("W32Time")
	if err != nil {
		return nil, err
	}
	handle, err := windows.OpenService(manager, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return nil, err
	}
	service := &mgr.Service{Name: "W32Time", Handle: handle}
	defer func() { _ = service.Close() }()
	before, err := service.Query()
	if err != nil {
		return nil, err
	}
	if before.State != svc.Running || before.ProcessId == 0 {
		return nil, invalidReply("W32Time is not running")
	}
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, before.ProcessId)
	if err != nil {
		return nil, err
	}
	defer func() { _ = windows.CloseHandle(process) }()
	// Modern W32Time exposes W32TIME_ALT. Identification allows caller identity
	// checks without permission to act as the caller. Verify the peer
	// before sending any RPC bytes through this handle.
	pipe, err := winio.DialPipeAccessImpLevel(ctx, `\\.\pipe\W32TIME_ALT`, windows.GENERIC_READ|windows.GENERIC_WRITE, winio.PipeImpLevelIdentification)
	if err != nil {
		return nil, err
	}
	defer func() { _ = pipe.Close() }()
	if err := checkPipeProcess(pipe, before.ProcessId); err != nil {
		return nil, err
	}
	status, err := QueryStatus(ctx, pipe)
	if err != nil {
		return nil, err
	}
	after, err := service.Query()
	if err != nil {
		return nil, err
	}
	if after.State != svc.Running || after.ProcessId != before.ProcessId {
		return nil, invalidReply("W32Time process changed")
	}
	wait, err := windows.WaitForSingleObject(process, 0)
	if err != nil {
		return nil, err
	}
	if wait != uint32(windows.WAIT_TIMEOUT) {
		return nil, invalidReply("W32Time process stopped")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return status, nil
}

func checkPipeProcess(pipe net.Conn, expected uint32) error {
	fd, ok := pipe.(interface{ Fd() uintptr })
	if !ok {
		return invalidReply("pipe does not expose its handle")
	}
	var actual uint32
	if err := windows.GetNamedPipeServerProcessId(windows.Handle(fd.Fd()), &actual); err != nil {
		return err
	}
	if actual == 0 || actual != expected {
		return invalidReply("pipe is not owned by W32Time")
	}
	return nil
}
