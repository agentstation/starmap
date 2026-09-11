package w32timerpc

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestFragmentedQueryStatus(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	done := make(chan error, 1)
	go func() { done <- serveStatus(server, true, nil) }()
	status, err := queryStatus(t.Context(), client, nil)
	if err != nil || status == nil || status.Source != "test-source" || status.TimeLastGoodSync != 100000 {
		t.Fatalf("fragmented response: %+v, %v", status, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCancellationAfterQueryDispatch(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	c := &tracked{Conn: client}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	serverDone := make(chan error, 1)
	go func() { serverDone <- serveStatus(server, false, func() { close(started); <-ctx.Done() }) }()
	done := make(chan error, 1)
	go func() { _, err := queryStatus(ctx, c, nil); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("query did not reach the status operation")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled query succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("dispatched query ignored cancellation")
	}
	if !c.closed.Load() {
		t.Fatal("canceled query left its stream open")
	}
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("server stream did not close")
	}
}
