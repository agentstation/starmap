package auth

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
)

// Secret SDK tests run serially because they replace the default HTTP transport.
func localSecretSDKTransport(t *testing.T, handler http.Handler, hosts ...string) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	original := http.DefaultTransport
	transport := original.(*http.Transport).Clone()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	transport.TLSClientConfig.InsecureSkipVerify = false
	transport.TLSClientConfig.RootCAs = roots
	transport.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
	transport.Proxy = nil
	transport.DialTLSContext = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		for _, allowed := range hosts {
			if host == allowed {
				return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
			}
		}
		t.Errorf("SDK attempted an unexpected host: %s", host)
		return nil, fmt.Errorf("unexpected SDK fixture host")
	}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original; transport.CloseIdleConnections() })
}

func clearSecretSDKEnvironment(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "AWS_") || strings.HasPrefix(name, "AZURE_") {
			t.Setenv(name, "")
		}
	}
}

func secretSDKFixtureFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	return path
}
