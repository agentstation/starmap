package w32timerpc

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

type bufferConn struct {
	net.Conn
	*bytes.Reader
	deadline time.Time
}

func (c *bufferConn) Read(b []byte) (int, error)         { return c.Reader.Read(b) }
func (c *bufferConn) SetDeadline(t time.Time) error      { c.deadline = t; return nil }
func (c *bufferConn) SetReadDeadline(t time.Time) error  { c.deadline = t; return nil }
func (c *bufferConn) SetWriteDeadline(t time.Time) error { c.deadline = t; return nil }

func TestFrameBoundsBeforeDecoder(t *testing.T) {
	for name, mutate := range map[string]func([]byte){
		"oversize fragment": func(p []byte) { binary.LittleEndian.PutUint16(p[8:], maxFragmentBytes+1) },
		"short fragment":    func(p []byte) { binary.LittleEndian.PutUint16(p[8:], 15) },
		"large allocation":  func(p []byte) { binary.LittleEndian.PutUint32(p[16:], ^uint32(0)) },
		"auth trailer":      func(p []byte) { binary.LittleEndian.PutUint16(p[10:], 1) },
		"wrong encoding":    func(p []byte) { p[4] = 0 },
		"unexpected type":   func(p []byte) { p[2] = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			p := packet(2, 1, make([]byte, 8))
			mutate(p)
			c := &boundedStream{Conn: &bufferConn{Reader: bytes.NewReader(p)}}
			var out [100]byte
			if n, err := c.Read(out[:]); n != 0 || err == nil {
				t.Fatalf("invalid frame reached decoder: %d, %v", n, err)
			}
			if n, err := c.Read(out[:]); n != 0 || err == nil {
				t.Fatal("failed read did not remain failed")
			}
		})
	}
}

func TestTotalReplyBound(t *testing.T) {
	p := packet(2, 1, make([]byte, maxFragmentBytes-16))
	c := &boundedStream{Conn: &bufferConn{Reader: bytes.NewReader(bytes.Repeat(p, 5))}}
	n, err := io.Copy(io.Discard, c)
	if n != maxReplyBytes || err == nil {
		t.Fatalf("total read %d, %v", n, err)
	}
}

func TestDeadlineCannotBeExtended(t *testing.T) {
	end := time.Now().Add(time.Second)
	base := &bufferConn{}
	c := &boundedStream{Conn: base, deadline: end}
	for _, set := range []func(time.Time) error{c.SetDeadline, c.SetReadDeadline, c.SetWriteDeadline} {
		for _, requested := range []time.Time{{}, end.Add(time.Hour)} {
			if err := set(requested); err != nil {
				t.Fatal(err)
			}
			if !base.deadline.Equal(end) {
				t.Fatal("deadline extended")
			}
		}
		earlier := end.Add(-time.Millisecond)
		if err := set(earlier); err != nil {
			t.Fatal(err)
		}
		if !base.deadline.Equal(earlier) {
			t.Fatal("earlier deadline lost")
		}
	}
}
