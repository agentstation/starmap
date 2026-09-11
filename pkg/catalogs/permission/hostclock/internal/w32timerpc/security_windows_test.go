package w32timerpc

import (
	"strings"
	"testing"

	mgmt "github.com/oiweiwei/go-msrpc/msrpc/mgmt/mgmt/v1"
)

func TestWindowsPrincipalDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply *mgmt.InquirePrincNameResponse
		want  string
	}{
		{"missing", nil, "reply is missing"},
		{"failed", &mgmt.InquirePrincNameResponse{Status: 5, PrincName: "private-principal"}, "query failed: 0x00000005"},
		{"failed-empty", &mgmt.InquirePrincNameResponse{Status: 5}, "query failed: 0x00000005"},
		{"unnamed", &mgmt.InquirePrincNameResponse{}, ""},
		{"long", &mgmt.InquirePrincNameResponse{PrincName: strings.Repeat("x", 1025)}, "exceeds the name limit"},
		{"valid", &mgmt.InquirePrincNameResponse{PrincName: "host/localhost"}, ""},
		{"boundary", &mgmt.InquirePrincNameResponse{PrincName: strings.Repeat("x", 1024)}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLocalPrincipal(tc.reply)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("principal diagnostic = %v, want %q", err, tc.want)
			}
			if tc.reply != nil && tc.reply.PrincName != "" && strings.Contains(err.Error(), tc.reply.PrincName) {
				t.Fatal("principal diagnostic disclosed the returned name")
			}
		})
	}
}
