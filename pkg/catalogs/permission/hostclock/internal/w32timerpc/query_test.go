package w32timerpc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oiweiwei/go-msrpc/dcerpc"
	"github.com/oiweiwei/go-msrpc/msrpc/w32t"
	w32time "github.com/oiweiwei/go-msrpc/msrpc/w32t/w32time/v4"
	"github.com/oiweiwei/go-msrpc/ndr"
)

type tracked struct {
	net.Conn
	closed atomic.Bool
}

func TestAuthenticationFailureClosesStream(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	c := &tracked{Conn: client}
	want := errors.New("authentication unavailable")
	_, err := queryStatus(t.Context(), c, func(context.Context, dcerpc.Conn) ([]dcerpc.Option, error) {
		return nil, want
	})
	if !errors.Is(err, want) || !c.closed.Load() {
		t.Fatalf("authentication failure did not close stream: %v, %v", err, c.closed.Load())
	}
}

func (c *tracked) Close() error { c.closed.Store(true); return c.Conn.Close() }

func TestQueryStatusWire(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	c := &tracked{Conn: client}
	serverDone := make(chan error, 1)
	go func() { serverDone <- serveStatus(server, false, nil) }()
	got, err := queryStatus(t.Context(), c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.LCState != 2 || got.TimeLastGoodSync != 100000 || got.RootDispersion != 12345 || got.Source != "test-source" {
		t.Fatalf("wrong decoded status: %+v", got)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	if !c.closed.Load() {
		t.Fatal("connection left open")
	}
}

func TestCanceledBindClosesStream(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	c := &tracked{Conn: client}
	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{})
	go func() { var b [1]byte; _, _ = server.Read(b[:]); close(started) }()
	done := make(chan error, 1)
	go func() { _, err := queryStatus(ctx, c, nil); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled query succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation failed to stop bind")
	}
	if !c.closed.Load() {
		t.Fatal("canceled query left stream open")
	}
}

func TestFixedStreamRefusesRedialAndAddressChange(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	defer client.Close()
	s := &fixedStream{conn: client}
	if _, err := s.DialContext(t.Context(), "tcp", "example.com:135"); err == nil {
		t.Fatal("accepted changed address")
	}
	if _, err := s.DialContext(t.Context(), "tcp", "127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DialContext(t.Context(), "tcp", "127.0.0.1:1"); err == nil {
		t.Fatal("redial accepted")
	}
}

func readPDU(c net.Conn) ([]byte, error) {
	header := make([]byte, 16)
	if _, err := io.ReadFull(c, header); err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint16(header[8:]))
	if n < len(header) {
		return nil, errors.New("short PDU")
	}
	pdu := make([]byte, n)
	copy(pdu, header)
	_, err := io.ReadFull(c, pdu[16:])
	return pdu, err
}

func packet(kind byte, callID uint32, body []byte) []byte {
	p := make([]byte, 16+len(body))
	p[0], p[2], p[3], p[4] = 5, kind, 3, 0x10
	binary.LittleEndian.PutUint16(p[8:], uint16(len(p)))
	binary.LittleEndian.PutUint32(p[12:], callID)
	copy(p[16:], body)
	return p
}

func serveStatus(c net.Conn, fragmented bool, onRequest func()) error {
	bind, err := readPDU(c)
	if err != nil {
		return err
	}
	if bind[2] != 11 || binary.LittleEndian.Uint16(bind[10:]) != 0 {
		return errors.New("not an anonymous bind")
	}
	// Accept the NDR32 presentation. Decline the optional feature presentation.
	body := make([]byte, 16+24*2)
	binary.LittleEndian.PutUint16(body, 4280)
	binary.LittleEndian.PutUint16(body[2:], 4280)
	binary.LittleEndian.PutUint32(body[4:], 1)
	body[12] = 2
	copy(body[20:], []byte{4, 0x5d, 0x88, 0x8a, 0xeb, 0x1c, 0xc9, 0x11, 0x9f, 0xe8, 8, 0, 0x2b, 0x10, 0x48, 0x60, 2, 0, 0, 0})
	binary.LittleEndian.PutUint16(body[40:], 2)
	if _, err := c.Write(packet(12, binary.LittleEndian.Uint32(bind[12:]), body)); err != nil {
		return err
	}
	request, err := readPDU(c)
	if err != nil {
		return err
	}
	if request[2] != 0 || binary.LittleEndian.Uint16(request[22:]) != 6 {
		return fmt.Errorf("unexpected request: %x", request)
	}
	if onRequest != nil {
		onRequest()
	}
	stub, err := ndr.Marshal(&w32time.QueryStatusResponse{StatusInfo: &w32t.StatusInfo{LCState: 2, TimeLastGoodSync: 100000, RootDispersion: 12345, Source: "test-source"}})
	if err != nil {
		return err
	}
	if fragmented {
		cut := len(stub) / 2
		for i, chunk := range [][]byte{stub[:cut], stub[cut:]} {
			body := make([]byte, 8+len(chunk))
			binary.LittleEndian.PutUint32(body, uint32(len(stub)))
			copy(body[4:6], request[20:22])
			copy(body[8:], chunk)
			pdu := packet(2, binary.LittleEndian.Uint32(request[12:]), body)
			pdu[3] = byte(i + 1)
			if _, err := c.Write(pdu); err != nil {
				return err
			}
		}
		return nil
	}
	response := make([]byte, 8+len(stub))
	binary.LittleEndian.PutUint32(response, uint32(len(stub)))
	copy(response[4:6], request[20:22])
	copy(response[8:], stub)
	_, err = c.Write(packet(2, binary.LittleEndian.Uint32(request[12:]), response))
	return err
}
