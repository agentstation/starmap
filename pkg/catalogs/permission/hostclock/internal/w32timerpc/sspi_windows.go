package w32timerpc

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/oiweiwei/go-msrpc/ssp/gssapi"
	"golang.org/x/sys/windows"
)

// These values and layouts follow the Windows SDK sspi.h user-mode ABI.
const (
	sspiContinue         = 0x00090312
	sspiComplete         = 0x00090313
	sspiCompleteContinue = 0x00090314
	sspiPrivacyFlags     = 0x00000004 | 0x00000008 | 0x00000010 | 0x00010000 | 0x00020000
	sspiContextFlags     = sspiPrivacyFlags | 0x00000200 | 0x00000800
	sspiNativeDREP       = 0x00000010
	sspiData             = 1
	sspiToken            = 2
	sspiPadding          = 9
	sspiReadOnly         = 0x80000000
	sspiReadOnlyChecksum = 0x10000000
	maxSecurityTrailer   = 4096
)

type securityHandle struct{ lower, upper uintptr }

func invalidSecurityHandle() securityHandle { return securityHandle{^uintptr(0), ^uintptr(0)} }
func (h securityHandle) valid() bool        { return h.lower != ^uintptr(0) && h.upper != ^uintptr(0) }

type securityBuffer struct {
	size uint32
	kind uint32
	data *byte
}

//nolint:gosec // G115: Callers bound each input to maxFragmentBytes before constructing its native descriptor.
func makeSecurityBuffer(kind uint32, data []byte) securityBuffer {
	var pointer *byte
	if len(data) != 0 {
		pointer = &data[0]
	}
	return securityBuffer{size: uint32(len(data)), kind: kind, data: pointer}
}

type securityBuffers struct {
	version uint32
	count   uint32
	data    *securityBuffer
}

//nolint:gosec // G115: Each SSPI call supplies either one handshake buffer or five packet buffers.
func describeSecurityBuffers(buffers []securityBuffer) securityBuffers {
	var pointer *securityBuffer
	if len(buffers) != 0 {
		pointer = &buffers[0]
	}
	return securityBuffers{count: uint32(len(buffers)), data: pointer}
}

type securitySizes struct{ maxToken, signature, block, trailer uint32 }

type securityAPI struct {
	acquire, initialize, complete, query, encrypt, decrypt, release, remove *windows.LazyProc
}

func loadSecurityAPI() (*securityAPI, error) {
	dll := windows.NewLazySystemDLL("secur32.dll")
	api := &securityAPI{
		acquire: dll.NewProc("AcquireCredentialsHandleW"), initialize: dll.NewProc("InitializeSecurityContextW"),
		complete: dll.NewProc("CompleteAuthToken"), query: dll.NewProc("QueryContextAttributesW"),
		encrypt: dll.NewProc("EncryptMessage"), decrypt: dll.NewProc("DecryptMessage"),
		release: dll.NewProc("FreeCredentialsHandle"), remove: dll.NewProc("DeleteSecurityContext"),
	}
	for _, proc := range []*windows.LazyProc{api.acquire, api.initialize, api.complete, api.query, api.encrypt, api.decrypt, api.release, api.remove} {
		if err := proc.Find(); err != nil {
			return nil, err
		}
	}
	return api, nil
}

// nativeSecurity owns ambient credentials and one identification-only context.
// Native work starts after pipe identity validation, within the observation worker.
type nativeSecurity struct {
	mu         sync.Mutex
	api        *securityAPI
	credential securityHandle
	context    securityHandle
	target     *uint16
	sizes      securitySizes
	ready      bool
	closed     bool
	steps      uint32
	sent       uint32
	received   uint32
}

//nolint:gosec // G103: UTF-16 and handle pointers follow the SSPI ABI and remain live across the call.
func newNativeSecurity(ctx context.Context, target string) (*nativeSecurity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if target == "" || len(target) > 1024 {
		return nil, invalidReply("invalid local service principal")
	}
	name, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return nil, err
	}
	api, err := loadSecurityAPI()
	if err != nil {
		return nil, err
	}
	s := &nativeSecurity{api: api, credential: invalidSecurityHandle(), context: invalidSecurityHandle(), target: name}
	pkg, err := windows.UTF16PtrFromString("Negotiate")
	if err != nil {
		return nil, err
	}
	// A null principal and auth-data pointer select the current Windows identity.
	var expiry windows.Filetime
	status, _, _ := api.acquire.Call(0, uintptr(unsafe.Pointer(pkg)), 2, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&s.credential)), uintptr(unsafe.Pointer(&expiry)))
	runtime.KeepAlive(pkg)
	if (status & 0xffffffff) != 0 {
		s.close()
		return nil, securityStatusError("credentials", status)
	}
	if !s.credential.valid() {
		s.close()
		return nil, invalidReply("Windows returned invalid credentials")
	}
	if err := ctx.Err(); err != nil {
		s.close()
		return nil, err
	}
	return s, nil
}

func securityStatusError(stage string, status uintptr) error {
	return invalidReply(fmt.Sprintf("Windows local authentication %s failed: 0x%08x", stage, (status & 0xffffffff)))
}

//nolint:gosec // G103: SSPI receives pointers to its valid handles under exclusive ownership.
func (s *nativeSecurity) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.ready = false
	if s.context.valid() {
		_, _, _ = s.api.remove.Call(uintptr(unsafe.Pointer(&s.context)))
		s.context = invalidSecurityHandle()
	}
	if s.credential.valid() {
		_, _, _ = s.api.release.Call(uintptr(unsafe.Pointer(&s.credential)))
		s.credential = invalidSecurityHandle()
	}
	s.sizes = securitySizes{}
}

func (s *nativeSecurity) trailerSize() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int(s.sizes.trailer)
}

//nolint:gosec // G103: SSPI buffer descriptors use the SDK layout, bounded slices, and explicit lifetime retention.
func (s *nativeSecurity) step(ctx context.Context, input []byte) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if s.closed || s.ready || s.steps >= 8 || len(input) > maxFragmentBytes {
		return nil, false, invalidReply("invalid local authentication exchange")
	}
	s.steps++
	var previous *securityHandle
	if s.context.valid() {
		previous = &s.context
	}
	in := []securityBuffer{makeSecurityBuffer(sspiToken, input)}
	inDesc := describeSecurityBuffers(in)
	var inPtr *securityBuffers
	if len(input) != 0 {
		inPtr = &inDesc
	}
	output := make([]byte, maxFragmentBytes)
	out := []securityBuffer{makeSecurityBuffer(sspiToken, output)}
	outDesc := describeSecurityBuffers(out)
	var flags uint32
	var expiry windows.Filetime
	status, _, _ := s.api.initialize.Call(uintptr(unsafe.Pointer(&s.credential)), uintptr(unsafe.Pointer(previous)),
		uintptr(unsafe.Pointer(s.target)), sspiContextFlags, 0, sspiNativeDREP, uintptr(unsafe.Pointer(inPtr)), 0,
		uintptr(unsafe.Pointer(&s.context)), uintptr(unsafe.Pointer(&outDesc)), uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&expiry)))
	runtime.KeepAlive(input)
	runtime.KeepAlive(in)
	runtime.KeepAlive(output)
	runtime.KeepAlive(out)
	if (status&0xffffffff) != 0 && (status&0xffffffff) != sspiContinue && (status&0xffffffff) != sspiComplete && (status&0xffffffff) != sspiCompleteContinue {
		return nil, false, securityStatusError("exchange", status)
	}
	if !s.context.valid() {
		return nil, false, invalidReply("Windows returned an invalid security context")
	}
	if (status&0xffffffff) == sspiComplete || (status&0xffffffff) == sspiCompleteContinue {
		completed, _, _ := s.api.complete.Call(uintptr(unsafe.Pointer(&s.context)), uintptr(unsafe.Pointer(&outDesc)))
		if (completed & 0xffffffff) != 0 {
			return nil, false, securityStatusError("completion", completed)
		}
	}
	if out[0].data != &output[0] || out[0].size > uint32(len(output)) || out[0].kind != sspiToken {
		return nil, false, invalidReply("invalid Windows authentication output")
	}
	complete := (status&0xffffffff) == 0 || (status&0xffffffff) == sspiComplete
	if complete {
		if flags&sspiPrivacyFlags != sspiPrivacyFlags || flags&(0x00000001|0x00040000) != 0 {
			return nil, false, invalidReply("Windows did not grant private identification-only authentication")
		}
		queried, _, _ := s.api.query.Call(uintptr(unsafe.Pointer(&s.context)), 0, uintptr(unsafe.Pointer(&s.sizes)))
		if (queried & 0xffffffff) != 0 {
			return nil, false, securityStatusError("packet sizes", queried)
		}
		if s.sizes.trailer == 0 || s.sizes.trailer > maxSecurityTrailer || s.sizes.block > 16 {
			return nil, false, invalidReply("unsupported Windows authentication packet sizes")
		}
		s.ready = true
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return output[:out[0].size], complete, nil
}

//nolint:gosec // G103: SSPI receives bounded packet buffers that remain live throughout the native call.
func (s *nativeSecurity) protect(ctx context.Context, token *gssapi.MessageTokenEx, decrypt bool) (*gssapi.MessageTokenEx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.closed || !s.ready || token == nil || token.QoP != 0 || len(token.Payloads) != 3 {
		return nil, invalidReply("invalid local authentication packet")
	}
	signature := token.Signature
	if decrypt {
		if len(signature) != int(s.sizes.trailer) {
			return nil, invalidReply("invalid local authentication signature size")
		}
	} else {
		signature = make([]byte, s.sizes.trailer)
	}
	buffers := []securityBuffer{makeSecurityBuffer(sspiToken, signature)}
	total := len(signature)
	for i, payload := range token.Payloads {
		if payload == nil || len(payload.Payload) > maxFragmentBytes-total {
			return nil, invalidReply("excessive local authentication packet")
		}
		total += len(payload.Payload)
		kind := uint32(sspiData | sspiReadOnly)
		if i == 1 {
			if payload.Capabilities&(gssapi.Confidentiality|gssapi.Integrity) != gssapi.Confidentiality|gssapi.Integrity {
				return nil, invalidReply("local RPC payload requires privacy")
			}
			kind = sspiData
		} else if payload.Capabilities&gssapi.Confidentiality != 0 {
			return nil, invalidReply("local RPC header cannot be encrypted")
		} else if payload.Capabilities&gssapi.Integrity != 0 {
			kind = sspiData | sspiReadOnlyChecksum
		}
		buffers = append(buffers, makeSecurityBuffer(kind, payload.Payload))
	}
	buffers = append(buffers, makeSecurityBuffer(sspiPadding, nil))
	desc := describeSecurityBuffers(buffers)
	var status uintptr
	var qop uint32
	if decrypt {
		status, _, _ = s.api.decrypt.Call(uintptr(unsafe.Pointer(&s.context)), uintptr(unsafe.Pointer(&desc)),
			uintptr(s.received), uintptr(unsafe.Pointer(&qop)))
		s.received++
	} else {
		status, _, _ = s.api.encrypt.Call(uintptr(unsafe.Pointer(&s.context)), 0, uintptr(unsafe.Pointer(&desc)), uintptr(s.sent))
		s.sent++
	}
	runtime.KeepAlive(buffers)
	runtime.KeepAlive(token)
	runtime.KeepAlive(signature)
	if (status & 0xffffffff) != 0 {
		s.ready = false
		return nil, securityStatusError("packet privacy", status)
	}
	if qop != 0 || buffers[0].size != s.sizes.trailer || buffers[0].data != &signature[0] || buffers[4].size != 0 {
		s.ready = false
		return nil, invalidReply("Windows changed the RPC privacy contract")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	token.Signature = signature
	return token, nil
}
