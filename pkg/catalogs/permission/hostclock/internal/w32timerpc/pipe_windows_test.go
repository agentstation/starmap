package w32timerpc

import (
	"context"
	"crypto/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func TestWindowsPipePeerIdentity(t *testing.T) {
	name := `\\.\pipe\starmap-clock-` + rand.Text()
	listener, err := winio.ListenPipe(name, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	type accepted struct {
		conn net.Conn
		err  error
	}
	done := make(chan accepted, 1)
	go func() { conn, err := listener.Accept(); done <- accepted{conn, err} }()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	client, err := winio.DialPipeContext(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case server := <-done:
		if server.err != nil {
			t.Fatal(server.err)
		}
		defer server.conn.Close()
	case <-ctx.Done():
		t.Fatal("pipe accept did not complete")
	}
	if err := checkPipeProcess(client, windows.GetCurrentProcessId()); err != nil {
		t.Fatalf("correct pipe process rejected: %v", err)
	}
	if err := checkPipeProcess(client, 0); err == nil {
		t.Fatal("wrong pipe process accepted")
	}
}

func TestWindowsLocalTimeService(t *testing.T) {
	status, err := ObserveStatus(t.Context())
	if err != nil {
		if os.Getenv("STARMAP_WINDOWS_CLOCK_FIXTURE_REQUIRED") == "1" {
			t.Fatalf("required local W32Time fixture: %v", err)
		}
		t.Skipf("local W32Time access is unavailable: %v", err)
	}
	if status == nil || status.Size == 0 {
		t.Fatalf("invalid native status: %+v", status)
	}
	t.Logf("W32Time status: size=%d state=%d leap=%d stratum=%d last_sync=%d age_ticks=%d dispersion_ticks=%d", status.Size, status.LCState, status.LeapIndicator, status.Stratum, status.LastSyncTicks, status.TimeLastGoodSync, status.RootDispersion)
}
