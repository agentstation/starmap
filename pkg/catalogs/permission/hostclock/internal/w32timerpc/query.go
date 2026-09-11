package w32timerpc

import (
	"context"
	"github.com/agentstation/starmap/pkg/errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/oiweiwei/go-msrpc/dcerpc"
	"github.com/oiweiwei/go-msrpc/msrpc/w32t"
	w32time "github.com/oiweiwei/go-msrpc/msrpc/w32t/w32time/v4"
)

const streamBinding = "ncacn_ip_tcp:127.0.0.1[1]"

func invalidReply(message string) error {
	return &errors.ValidationError{Field: "permission_clock", Message: message}
}

type fixedStream struct {
	conn net.Conn
	used atomic.Bool
}

func (s *fixedStream) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if network != "tcp" || address != "127.0.0.1:1" || !s.used.CompareAndSwap(false, true) {
		return nil, invalidReply("refused another transport")
	}
	return s.conn, nil
}

type noDNS struct{}

func (noDNS) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return nil, invalidReply("DNS is prohibited")
}

// QueryStatus reads W32Time status from a stream. The caller must check its peer first.
// It owns and closes the stream. It invokes only the generated QueryStatus operation.
func QueryStatus(ctx context.Context, raw net.Conn) (*w32t.StatusInfo, error) {
	if ctx == nil || raw == nil {
		return nil, invalidReply("a W32Time observation context and stream are required")
	}
	defer func() { _ = raw.Close() }()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := raw.SetDeadline(deadline); err != nil {
		return nil, err
	}
	stream := &boundedStream{Conn: raw, deadline: deadline}
	conn, err := dcerpc.Dial(ctx, streamBinding, dcerpc.WithDialer(&fixedStream{conn: stream}), dcerpc.WithDNSResolver(noDNS{}), dcerpc.WithTimeout(2*time.Second), dcerpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	client, err := w32time.NewW32TimeClient(ctx, conn, dcerpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	response, err := client.QueryStatus(ctx, &w32time.QueryStatusRequest{})
	if err != nil {
		return nil, err
	}
	if response == nil || response.StatusInfo == nil {
		return nil, invalidReply("missing status")
	}
	return response.StatusInfo, nil
}
