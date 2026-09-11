//go:build !windows

package w32timerpc

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestQueryStatusRefusesUnsupportedHostBeforeTransport(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	t.Cleanup(func() { _ = server.Close() })
	if err := server.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	status, err := QueryStatus(t.Context(), client)
	if status != nil || err == nil || !strings.Contains(err.Error(), "requires Windows") {
		t.Fatalf("unsupported host returned status=%v error=%v", status, err)
	}
	var payload [1]byte
	if count, err := server.Read(payload[:]); count != 0 || err != io.EOF {
		t.Fatalf("unsupported host sent bytes or left the stream open: count=%d error=%v", count, err)
	}
}
