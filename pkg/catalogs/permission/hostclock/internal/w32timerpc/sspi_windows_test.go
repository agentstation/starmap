package w32timerpc

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"testing"
	"unsafe"

	"github.com/oiweiwei/go-msrpc/ssp/gssapi"
	"golang.org/x/sys/windows"
)

func TestWindowsSecurityLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := newNativeSecurity(ctx, "host/localhost"); err == nil {
		t.Fatal("canceled credential acquisition succeeded")
	}
	s, err := newNativeSecurity(t.Context(), "host/localhost")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.close)
	if _, _, err := s.step(t.Context(), bytes.Repeat([]byte{0}, maxFragmentBytes+1)); err == nil {
		t.Fatal("excessive input token accepted")
	}
	if _, _, err := s.step(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	s.close()
	s.close()
	if s.credential.valid() || s.context.valid() || !s.closed {
		t.Fatal("native security handles survived close")
	}
	if _, _, err := s.step(t.Context(), nil); err == nil {
		t.Fatal("closed native context accepted a token")
	}
}

func TestWindowsSecurityPacketPrivacy(t *testing.T) {
	for _, name := range []string{"roundtrip", "body tamper", "header tamper", "signature tamper", "replay"} {
		t.Run(name, func(t *testing.T) {
			client, server := nativeSecurityPair(t)
			message := &gssapi.MessageTokenEx{Payloads: []*gssapi.PayloadEx{
				{Payload: []byte("header"), Capabilities: gssapi.Integrity},
				{Payload: []byte("private status bytes"), Capabilities: gssapi.Integrity | gssapi.Confidentiality},
				{Payload: make([]byte, 8), Capabilities: gssapi.Integrity},
			}}
			sealed, err := client.protect(t.Context(), message, false)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(sealed.Payloads[1].Payload, []byte("private status bytes")) {
				t.Fatal("packet body remained plaintext")
			}
			copyToken := func() *gssapi.MessageTokenEx {
				copy := &gssapi.MessageTokenEx{Signature: bytes.Clone(sealed.Signature)}
				for _, payload := range sealed.Payloads {
					copy.Payloads = append(copy.Payloads, &gssapi.PayloadEx{Payload: bytes.Clone(payload.Payload), Capabilities: payload.Capabilities})
				}
				return copy
			}
			incoming := copyToken()
			switch name {
			case "body tamper":
				incoming.Payloads[1].Payload[0] ^= 1
			case "header tamper":
				incoming.Payloads[0].Payload[0] ^= 1
			case "signature tamper":
				incoming.Signature[0] ^= 1
			}
			opened, err := server.protect(t.Context(), incoming, true)
			if name != "roundtrip" && name != "replay" {
				if err == nil || server.ready {
					t.Fatal("tampered packet accepted or context remained usable")
				}
				return
			}
			if err != nil || !bytes.Equal(opened.Payloads[1].Payload, []byte("private status bytes")) {
				t.Fatalf("packet privacy roundtrip: %v", err)
			}
			if name == "replay" {
				if _, err := server.protect(t.Context(), copyToken(), true); err == nil {
					t.Fatal("replayed packet accepted")
				}
			}
		})
	}
}

// nativeSecurityPair exercises the real Windows SSP on both sides of one exchange.
func nativeSecurityPair(t *testing.T) (*nativeSecurity, *nativeSecurity) {
	t.Helper()
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	client, err := newNativeSecurity(t.Context(), "host/"+host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.close)
	server := &nativeSecurity{api: client.api, credential: invalidSecurityHandle(), context: invalidSecurityHandle()}
	t.Cleanup(server.close)
	pkg, err := windows.UTF16PtrFromString("Negotiate")
	if err != nil {
		t.Fatal(err)
	}
	var expiry windows.Filetime
	status, _, _ := server.api.acquire.Call(0, uintptr(unsafe.Pointer(pkg)), 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&server.credential)), uintptr(unsafe.Pointer(&expiry)))
	if uint32(status) != 0 {
		t.Fatal(securityStatusError("test server credentials", status))
	}
	accept := windows.NewLazySystemDLL("secur32.dll").NewProc("AcceptSecurityContext")
	if err := accept.Find(); err != nil {
		t.Fatal(err)
	}
	var input []byte
	for range 8 {
		output, complete, err := client.step(t.Context(), input)
		if err != nil {
			t.Fatal(err)
		}
		if complete && server.ready && len(output) == 0 {
			return client, server
		}
		var previous *securityHandle
		if server.context.valid() {
			previous = &server.context
		}
		in := []securityBuffer{makeSecurityBuffer(sspiToken, output)}
		inDesc := describeSecurityBuffers(in)
		input = make([]byte, maxFragmentBytes)
		out := []securityBuffer{makeSecurityBuffer(sspiToken, input)}
		outDesc := describeSecurityBuffers(out)
		var flags uint32
		// Server flags request replay checks, sequence checks, privacy, and DCE framing.
		status, _, _ = accept.Call(uintptr(unsafe.Pointer(&server.credential)), uintptr(unsafe.Pointer(previous)),
			uintptr(unsafe.Pointer(&inDesc)), 0x00000004|0x00000008|0x00000010|0x00000200|0x00000800|0x00020000,
			sspiNativeDREP, uintptr(unsafe.Pointer(&server.context)), uintptr(unsafe.Pointer(&outDesc)),
			uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&expiry)))
		runtime.KeepAlive(output)
		runtime.KeepAlive(in)
		runtime.KeepAlive(out)
		if uint32(status) == sspiComplete || uint32(status) == sspiCompleteContinue {
			rc, _, _ := server.api.complete.Call(uintptr(unsafe.Pointer(&server.context)), uintptr(unsafe.Pointer(&outDesc)))
			if uint32(rc) != 0 {
				t.Fatal(securityStatusError("test server completion", rc))
			}
		}
		if uint32(status) != 0 && uint32(status) != sspiContinue && uint32(status) != sspiComplete && uint32(status) != sspiCompleteContinue {
			t.Fatal(securityStatusError("test server exchange", status))
		}
		if out[0].size > uint32(len(input)) || out[0].data != unsafe.SliceData(input) {
			t.Fatal("test server returned excessive output")
		}
		input = input[:out[0].size]
		server.ready = uint32(status) == 0 || uint32(status) == sspiComplete
		if server.ready {
			rc, _, _ := server.api.query.Call(uintptr(unsafe.Pointer(&server.context)), 0, uintptr(unsafe.Pointer(&server.sizes)))
			if uint32(rc) != 0 || server.sizes.trailer == 0 || server.sizes.trailer > maxSecurityTrailer {
				t.Fatal("test server packet sizes unavailable")
			}
		}
		if complete && server.ready && len(input) == 0 {
			return client, server
		}
	}
	t.Fatal("native authentication exceeded eight exchanges")
	return nil, nil
}
