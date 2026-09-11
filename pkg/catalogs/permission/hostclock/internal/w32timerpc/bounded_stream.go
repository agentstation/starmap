package w32timerpc

import (
	"encoding/binary"
	"io"
	"net"
	"time"
)

const maxReplyBytes = 64 << 10
const maxFragmentBytes = 16 << 10

// boundedStream validates frame limits before the RPC decoder sees size hints.
// Each instance belongs to one observation and one reader.
type boundedStream struct {
	net.Conn
	deadline      time.Time
	buffer        [maxFragmentBytes]byte
	ready         []byte
	received      int
	err           error
	authenticated bool
}

func (c *boundedStream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if c.err != nil {
		return 0, c.err
	}
	if len(c.ready) == 0 {
		c.err = c.readFragment()
		if c.err != nil {
			return 0, c.err
		}
	}
	n := copy(p, c.ready)
	c.ready = c.ready[n:]
	return n, nil
}

func (c *boundedStream) readFragment() error {
	h := c.buffer[:16]
	if _, err := io.ReadFull(c.Conn, h); err != nil {
		return err
	}
	size := int(binary.LittleEndian.Uint16(h[8:]))
	authSize := int(binary.LittleEndian.Uint16(h[10:]))
	if h[0] != 5 || h[1] != 0 || h[4] != 0x10 || h[5] != 0 || h[6] != 0 || h[7] != 0 ||
		size < 24 || size > len(c.buffer) || size > maxReplyBytes-c.received ||
		(authSize != 0 && (!c.authenticated || authSize > size-32)) {
		return invalidReply("invalid or excessive RPC frame")
	}
	if h[2] != 12 && h[2] != 2 && h[2] != 3 && (!c.authenticated || h[2] != 15) {
		return invalidReply("unexpected RPC response type")
	}
	frame := c.buffer[:size]
	if _, err := io.ReadFull(c.Conn, frame[16:]); err != nil {
		return err
	}
	if err := validateAuthTrailer(frame, authSize); err != nil {
		return err
	}
	if h[2] == 12 || h[2] == 15 {
		if size < 32 || binary.LittleEndian.Uint16(frame[16:]) < 1024 || binary.LittleEndian.Uint16(frame[16:]) > maxFragmentBytes ||
			binary.LittleEndian.Uint16(frame[18:]) < 1024 || binary.LittleEndian.Uint16(frame[18:]) > maxFragmentBytes {
			return invalidReply("invalid RPC fragment negotiation")
		}
	} else if binary.LittleEndian.Uint32(frame[16:]) > maxReplyBytes {
		return invalidReply("excessive RPC allocation hint")
	}
	c.received += size
	c.ready = frame
	return nil
}

func validateAuthTrailer(frame []byte, authSize int) error {
	if authSize == 0 {
		return nil
	}
	minimum := 24
	if frame[2] == 12 || frame[2] == 15 {
		minimum = 32
	}
	if authSize > len(frame)-minimum-8 {
		return invalidReply("RPC authentication overlaps the response header")
	}
	trailer := frame[len(frame)-authSize-8:]
	if trailer[0] != 9 || trailer[1] != 6 || trailer[3] != 0 || int(trailer[2]) > len(frame)-authSize-minimum-8 {
		return invalidReply("invalid RPC authentication trailer")
	}
	return nil
}

func (c *boundedStream) limit(t time.Time) time.Time {
	if t.IsZero() || t.After(c.deadline) {
		return c.deadline
	}
	return t
}
func (c *boundedStream) SetDeadline(t time.Time) error     { return c.Conn.SetDeadline(c.limit(t)) }
func (c *boundedStream) SetReadDeadline(t time.Time) error { return c.Conn.SetReadDeadline(c.limit(t)) }
func (c *boundedStream) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(c.limit(t))
}
