// Package permission issues bounded catalog permission receipts from current authority storage.
package permission

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// ClockReading binds a time to its cached, qualified uncertainty.
type ClockReading struct {
	Time        time.Time
	Uncertainty time.Duration
	Known       bool
}

// IssuerConfig selects one authority and its bounded receipt lifetime.
// Clock reads cached evidence for the returned time and supports concurrent calls.
// The caller owns clock qualification and the current authority reader.
type IssuerConfig struct {
	AuthorityID string
	PolicyID    string
	// Lifetime zero selects the five-minute maximum profile.
	Lifetime time.Duration
	Clock    func() ClockReading
}

// Issuer issues receipts for explicitly selected authoritative storage.
// NewIssuer starts no I/O. It does not turn a subscriber into an authority.
// The owning publisher must enforce durable publication order and authorize this issuer.
type Issuer struct {
	reader  storage.AuthorityHeadReader
	config  IssuerConfig
	mu      sync.Mutex
	highest catalogs.CatalogAuthorityHead
	last    catalogs.CatalogPermissionEnvelope
}

// NewIssuer constructs a passive issuer for one authority and policy.
func NewIssuer(reader storage.AuthorityHeadReader, config IssuerConfig) (*Issuer, error) {
	if nilAuthorityReader(reader) {
		return nil, issuerError("current authority reader is required")
	}
	if config.Clock == nil {
		return nil, issuerError("qualified clock callback is required")
	}
	if err := catalogs.ValidateCatalogAuthorityIdentity(config.AuthorityID, config.PolicyID); err != nil {
		return nil, err
	}
	if config.Lifetime == 0 {
		config.Lifetime = catalogs.MaxCatalogPermissionValidity
	}
	if config.Lifetime < 0 || config.Lifetime > catalogs.MaxCatalogPermissionValidity {
		return nil, issuerError("receipt lifetime must be positive and within the supported maximum")
	}
	return &Issuer{reader: reader, config: config}, nil
}

// ReadPermission observes current authority before issuing a bounded receipt.
// Validity starts at the clock's earliest possible time before the storage read.
// Read delay and issuer uncertainty consume that interval. Consumers also subtract their own uncertainty.
// A retained receipt never receives a later expiry without another current storage observation.
func (i *Issuer) ReadPermission(ctx context.Context) (catalogs.CatalogPermissionEnvelope, error) {
	if i == nil || ctx == nil || i.config.Clock == nil || i.reader == nil {
		return catalogs.CatalogPermissionEnvelope{}, issuerError("a constructed issuer and context are required")
	}
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	before := i.config.Clock()
	if !before.valid() {
		return catalogs.CatalogPermissionEnvelope{}, issuerError("clock validity is unknown or outside its supported bound")
	}
	issued := before.Time.Add(-before.Uncertainty).UTC()
	readContext, cancel := context.WithTimeout(ctx, i.config.Lifetime)
	defer cancel()
	head, err := i.reader.CurrentAuthorityHead(readContext)
	if err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	if err := head.Validate(); err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	if head.AuthorityID != i.config.AuthorityID || head.PolicyID != i.config.PolicyID {
		return catalogs.CatalogPermissionEnvelope{}, issuerError("current head does not match the configured authority and policy")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.highest != (catalogs.CatalogAuthorityHead{}) {
		if err := i.highest.ValidateSuccessor(head); err != nil {
			return catalogs.CatalogPermissionEnvelope{}, err
		}
	}
	i.highest = head
	if err := readContext.Err(); err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	after := i.config.Clock()
	if !after.valid() {
		return catalogs.CatalogPermissionEnvelope{}, issuerError("clock validity changed during the authority observation")
	}
	receipt := catalogs.CatalogPermissionEnvelope{
		Version: catalogs.CatalogPermissionEnvelopeVersion, Head: head,
		IssuedAt: issued, ValidUntil: issued.Add(i.config.Lifetime),
	}
	if err := receipt.Validate(); err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	if !receipt.ValidAt(after.Time, after.Uncertainty, after.Known) {
		return catalogs.CatalogPermissionEnvelope{}, issuerError("authority observation exhausted its permission interval")
	}
	if i.last.Version != 0 {
		if !receipt.IssuedAt.After(i.last.IssuedAt) && head == i.last.Head {
			if i.last.ValidAt(after.Time, after.Uncertainty, after.Known) {
				return i.last, nil
			}
			return catalogs.CatalogPermissionEnvelope{}, issuerError("previous receipt cannot be renewed at this issue time")
		}
		if err := i.last.ValidateSuccessor(receipt); err != nil {
			return catalogs.CatalogPermissionEnvelope{}, err
		}
	}
	i.last = receipt
	return receipt, nil
}

func (r ClockReading) valid() bool {
	return r.Known && !r.Time.IsZero() && r.Uncertainty >= 0 && r.Uncertainty <= catalogs.MaxCatalogPermissionClockUncertainty
}

func issuerError(message string) error {
	return &errors.ConfigError{Component: "permission issuer", Message: message}
}

func nilAuthorityReader(reader storage.AuthorityHeadReader) bool {
	value := reflect.ValueOf(reader)
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
