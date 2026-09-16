package artifact

import (
	"testing"
	"time"
)

func TestPublicationReceiptClassifiesStaleEvidenceUnderExplicitPolicy(t *testing.T) {
	for _, test := range []struct {
		name    string
		age     time.Duration
		kind    string
		permit  bool
		allowed bool
	}{
		{"boundary", time.Hour, "retained", false, true},
		{"explicit-stale", time.Hour + time.Nanosecond, "stale_retained", true, true},
		{"no-permission", 2 * time.Hour, "stale_retained", false, false},
		{"hidden-staleness", 2 * time.Hour, "retained", true, false},
		{"false-staleness", time.Hour, "stale_retained", true, false},
		{"future-observation", -time.Hour, "retained", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			receipt := publicationReceiptFixture(t)
			receipt.FreshAcquisition = false
			row := &receipt.Sources[0]
			row.Policy.AllowStaleRetained = test.permit
			row.Attempt, row.EvidenceKind = "failed", test.kind
			row.Observation.ObservedAt = receipt.CompletedAt.Add(-test.age)
			encoded, err := EncodePublicationReceipt(receipt)
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%t, error=%v", test.allowed, err)
			}
			if test.allowed {
				decoded, err := DecodePublicationReceipt(encoded)
				if err != nil || decoded.Sources[0].EvidenceKind != test.kind || decoded.CompletedAt.Sub(decoded.Sources[0].Observation.ObservedAt) != test.age {
					t.Fatalf("receipt round trip changed status or age: %+v, %v", decoded, err)
				}
			}
		})
	}
}
