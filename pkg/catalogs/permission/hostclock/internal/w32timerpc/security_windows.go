package w32timerpc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/oiweiwei/go-msrpc/dcerpc"
	mgmt "github.com/oiweiwei/go-msrpc/msrpc/mgmt/mgmt/v1"
	"github.com/oiweiwei/go-msrpc/msrpc/w32t"
	"github.com/oiweiwei/go-msrpc/ssp/gssapi"
)

var localNegotiateOID = gssapi.OID{1, 3, 6, 1, 5, 5, 2}

// QueryStatus authenticates to a checked local pipe and reads W32Time status.
// It closes the stream and native security handles before returning.
func QueryStatus(ctx context.Context, raw net.Conn) (*w32t.StatusInfo, error) {
	factory := &localSecurityFactory{}
	defer factory.close()
	return queryStatus(ctx, raw, func(ctx context.Context, conn dcerpc.Conn) ([]dcerpc.Option, error) {
		client, err := mgmt.NewManagementClient(ctx, conn, dcerpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		// The checked local service supplies its principal on this same connection.
		principal, err := client.InquirePrincName(ctx, &mgmt.InquirePrincNameRequest{AuthnProto: 9, PrincNameSize: 1024})
		if err != nil {
			return nil, err
		}
		if err := validateLocalPrincipal(principal); err != nil {
			return nil, err
		}
		// An unnamed service still requires packet privacy on the checked pipe.
		// Bind the native target to this reply, not an inherited GSSAPI context.
		factory.target = principal.PrincName
		return []dcerpc.Option{dcerpc.WithMechanism(factory), dcerpc.WithTargetName(principal.PrincName),
			dcerpc.WithSecurtyProvider(dcerpc.AuthTypeGSSNegotiate), dcerpc.WithSeal(), dcerpc.Impersonate()}, nil
	})
}

func validateLocalPrincipal(principal *mgmt.InquirePrincNameResponse) error {
	if principal == nil {
		return invalidReply("local W32Time principal reply is missing")
	}
	if principal.Status != 0 {
		return invalidReply(fmt.Sprintf("local W32Time principal query failed: 0x%08x", principal.Status))
	}
	if len(principal.PrincName) > 1024 {
		return invalidReply("local W32Time principal exceeds the name limit")
	}
	return nil
}

// localSecurityFactory owns one ambient Windows security context per observation.
type localSecurityFactory struct {
	mu      sync.Mutex
	target  string
	session *localSecurity
	closed  bool
}

func (*localSecurityFactory) Type() gssapi.OID { return localNegotiateOID }
func (*localSecurityFactory) DefaultConfig(context.Context) (gssapi.MechanismConfig, error) {
	return localSecurityConfig{}, nil
}
func (f *localSecurityFactory) New(ctx context.Context) (gssapi.Mechanism, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed || f.session != nil {
		return nil, invalidReply("repeated local authentication context")
	}
	cc := gssapi.FromContext(ctx)
	required := gssapi.Confidentiality | gssapi.Integrity | gssapi.ReplayDetection | gssapi.Sequencing
	if cc.IsServer || cc.Capabilities&required != required ||
		cc.Capabilities&(gssapi.Delegation|gssapi.Anonymity) != 0 {
		return nil, invalidReply("local authentication requires packet privacy")
	}
	session, err := newNativeSecurity(ctx, f.target)
	if err != nil {
		return nil, err
	}
	f.session = &localSecurity{native: session}
	return f.session, nil
}
func (f *localSecurityFactory) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	if f.session != nil {
		f.session.native.close()
	}
}

type localSecurityConfig struct{}

func (localSecurityConfig) Type() gssapi.OID               { return localNegotiateOID }
func (c localSecurityConfig) Copy() gssapi.MechanismConfig { return c }

// localSecurity adapts Windows packet privacy to the RPC mechanism interface.
type localSecurity struct{ native *nativeSecurity }

func (*localSecurity) Type() gssapi.OID { return localNegotiateOID }
func (*localSecurity) Capabilities(context.Context) gssapi.Cap {
	return gssapi.Confidentiality | gssapi.Integrity | gssapi.ReplayDetection | gssapi.Sequencing
}
func (s *localSecurity) Init(ctx context.Context, token *gssapi.Token) (*gssapi.Token, error) {
	if token == nil {
		return nil, invalidReply("missing local authentication token")
	}
	output, complete, err := s.native.step(ctx, token.Payload)
	if err != nil {
		return nil, err
	}
	result := &gssapi.Token{Payload: output}
	if complete {
		return result, gssapi.ContextComplete(ctx)
	}
	return result, gssapi.ContextContinueNeeded(ctx)
}
func (*localSecurity) Accept(context.Context, *gssapi.Token) (*gssapi.Token, error) {
	return nil, invalidReply("local authentication cannot accept clients")
}
func (s *localSecurity) WrapSizeLimit(_ context.Context, limit int, _ bool) int {
	return limit - s.native.trailerSize()
}
func (s *localSecurity) WrapEx(ctx context.Context, token *gssapi.MessageTokenEx) (*gssapi.MessageTokenEx, error) {
	return s.native.protect(ctx, token, false)
}
func (s *localSecurity) UnwrapEx(ctx context.Context, token *gssapi.MessageTokenEx) (*gssapi.MessageTokenEx, error) {
	return s.native.protect(ctx, token, true)
}
func (*localSecurity) Wrap(context.Context, *gssapi.MessageToken) (*gssapi.MessageToken, error) {
	return nil, invalidReply("local authentication requires RPC packet buffers")
}
func (*localSecurity) Unwrap(context.Context, *gssapi.MessageToken) (*gssapi.MessageToken, error) {
	return nil, invalidReply("local authentication requires RPC packet buffers")
}
func (*localSecurity) MakeSignature(context.Context, *gssapi.MessageToken) (*gssapi.MessageToken, error) {
	return nil, invalidReply("local authentication requires packet privacy")
}
func (*localSecurity) VerifySignature(context.Context, *gssapi.MessageToken) error {
	return invalidReply("local authentication requires packet privacy")
}
func (*localSecurity) MakeSignatureEx(context.Context, *gssapi.MessageTokenEx) (*gssapi.MessageTokenEx, error) {
	return nil, invalidReply("local authentication requires packet privacy")
}
func (*localSecurity) VerifySignatureEx(context.Context, *gssapi.MessageTokenEx) error {
	return invalidReply("local authentication requires packet privacy")
}

var _ gssapi.MechanismFactory = (*localSecurityFactory)(nil)
var _ gssapi.Mechanism = (*localSecurity)(nil)
var _ gssapi.MechanismEx = (*localSecurity)(nil)
