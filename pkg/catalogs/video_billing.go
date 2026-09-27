package catalogs

import (
	"slices"
	"strconv"
	"strings"
)

// VideoContractSchemaVersion adds complete video billing and exact-model endpoints.
const VideoContractSchemaVersion uint64 = 19

// VideoBilling declares complete output-duration charges for the permitted inputs.
// OutputSecond supplies USD per provider-reported output second.
// Seconds and Sizes delimit this price contract, not all possible model inputs.
// There are no separate token, image, audio, quality, seed, or input charges.
// RequestCharge explicitly declares whether Operations.Request also applies.
// A missing contract is unknown and cannot authorize budgeted dispatch.
type VideoBilling struct {
	Basis          VideoBillingBasis `json:"basis" yaml:"basis"`
	DefaultSeconds int64             `json:"default_seconds" yaml:"default_seconds"`
	Seconds        []int64           `json:"seconds" yaml:"seconds"`
	DefaultSize    string            `json:"default_size" yaml:"default_size"`
	Sizes          []string          `json:"sizes" yaml:"sizes"`
	RequestCharge  *bool             `json:"request_charge" yaml:"request_charge"`
}

// VideoBillingBasis identifies the measured video charge unit.
type VideoBillingBasis string

// VideoBillingOutputSeconds charges measured output duration, including zero.
const VideoBillingOutputSeconds VideoBillingBasis = "output_seconds"

func (b *VideoBilling) validate() error {
	if b == nil {
		return nil
	}
	if b.Basis != VideoBillingOutputSeconds || b.RequestCharge == nil {
		return billingValidationError("videos", nil, "requires output_seconds and explicit request_charge")
	}
	if b.DefaultSeconds <= 0 || !slices.Contains(b.Seconds, b.DefaultSeconds) || !slices.Contains(b.Sizes, b.DefaultSize) {
		return billingValidationError("videos", nil, "defaults must belong to the permitted seconds and sizes")
	}
	seconds := make(map[int64]bool, len(b.Seconds))
	for _, value := range b.Seconds {
		if value <= 0 || seconds[value] {
			return billingValidationError("videos.seconds", value, "requires distinct positive seconds")
		}
		seconds[value] = true
	}
	sizes := make(map[string]bool, len(b.Sizes))
	for _, value := range b.Sizes {
		width, height, found := strings.Cut(value, "x")
		w, we := strconv.ParseInt(width, 10, 32)
		h, he := strconv.ParseInt(height, 10, 32)
		if !found || we != nil || he != nil || w <= 0 || h <= 0 || sizes[value] || strconv.FormatInt(w, 10)+"x"+strconv.FormatInt(h, 10) != value {
			return billingValidationError("videos.sizes", value, "requires distinct positive WIDTHxHEIGHT dimensions")
		}
		sizes[value] = true
	}
	return nil
}
