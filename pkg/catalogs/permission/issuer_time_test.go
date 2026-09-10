package permission

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type headReaderFunc func(context.Context) (catalogs.CatalogAuthorityHead, error)

func (f headReaderFunc) CurrentAuthorityHead(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
	return f(ctx)
}

func TestIssuerDelayAndClockBounds(t *testing.T) {
	for _, scenario := range []struct {
		name              string
		delay             time.Duration
		beforeUncertainty time.Duration
		afterUncertainty  time.Duration
		afterKnown        bool
		wantReceipt       bool
	}{
		{"immediate", 0, 0, 0, true, true},
		{"delayed", time.Minute, 0, 0, true, true},
		{"issuer uncertainty", time.Minute, 20 * time.Second, 20 * time.Second, true, true},
		{"exact expiry", 5 * time.Minute, 0, 0, true, false},
		{"expired", 6 * time.Minute, 0, 0, true, false},
		{"expiry with uncertainty", 250 * time.Second, 20 * time.Second, 30 * time.Second, true, false},
		{"lost qualification", time.Second, 0, 0, false, false},
		{"excessive uncertainty", time.Second, 0, 31 * time.Second, true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := storage.NewMemory()
			generation := issuerGeneration(t, "timed-publication", 1)
			if err := store.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			start := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
			reading := ClockReading{Time: start, Uncertainty: scenario.beforeUncertainty, Known: true}
			reader := headReaderFunc(func(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
				head, err := store.CurrentAuthorityHead(ctx)
				reading = ClockReading{Time: start.Add(scenario.delay), Uncertainty: scenario.afterUncertainty, Known: scenario.afterKnown}
				return head, err
			})
			issuer, err := NewIssuer(reader, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading { return reading }})
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := issuer.ReadPermission(t.Context())
			if !scenario.wantReceipt {
				if err == nil || receipt != (catalogs.CatalogPermissionEnvelope{}) {
					t.Fatalf("receipt=%+v error=%v", receipt, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			earliest := start.Add(-scenario.beforeUncertainty)
			if !receipt.IssuedAt.Equal(earliest) || !receipt.ValidUntil.Equal(earliest.Add(5*time.Minute)) {
				t.Fatalf("read delay extended the interval: %+v", receipt)
			}
			if receipt.ValidAt(receipt.ValidUntil.Add(-30*time.Second), 30*time.Second, true) {
				t.Fatal("consumer uncertainty did not shorten expiry")
			}
		})
	}
}

func TestIssuerConstructorIsPassiveAndRejectsInvalidInputs(t *testing.T) {
	calls := 0
	reader := headReaderFunc(func(context.Context) (catalogs.CatalogAuthorityHead, error) {
		calls++
		return catalogs.CatalogAuthorityHead{}, nil
	})
	config := IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading { calls++; return ClockReading{} }}
	if _, err := NewIssuer(reader, config); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("construction performed an observation")
	}
	for name, change := range map[string]func(*IssuerConfig){
		"authority absent":   func(c *IssuerConfig) { c.AuthorityID = "" },
		"policy absent":      func(c *IssuerConfig) { c.PolicyID = "" },
		"identity too long":  func(c *IssuerConfig) { c.AuthorityID = strings.Repeat("a", 257) },
		"identity control":   func(c *IssuerConfig) { c.PolicyID = "production\n" },
		"clock absent":       func(c *IssuerConfig) { c.Clock = nil },
		"negative lifetime":  func(c *IssuerConfig) { c.Lifetime = -time.Second },
		"excessive lifetime": func(c *IssuerConfig) { c.Lifetime = catalogs.MaxCatalogPermissionValidity + time.Nanosecond },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := config
			change(&candidate)
			if _, err := NewIssuer(reader, candidate); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
	if _, err := NewIssuer(nil, config); err == nil {
		t.Fatal("accepted absent reader")
	}
	var absent *storage.Memory
	if _, err := NewIssuer(absent, config); err == nil {
		t.Fatal("accepted typed nil reader")
	}
	if calls != 0 {
		t.Fatal("invalid constructor performed an observation")
	}
}

func TestIssuerUnknownClockAndCancellationAvoidStorage(t *testing.T) {
	calls := 0
	reader := headReaderFunc(func(context.Context) (catalogs.CatalogAuthorityHead, error) {
		calls++
		return catalogs.CatalogAuthorityHead{}, nil
	})
	for _, reading := range []ClockReading{
		{},
		{Time: time.Now(), Known: false},
		{Time: time.Now(), Known: true, Uncertainty: -time.Second},
		{Time: time.Now(), Known: true, Uncertainty: 31 * time.Second},
	} {
		issuer, err := NewIssuer(reader, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading { return reading }})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := issuer.ReadPermission(t.Context()); err == nil {
			t.Fatal("unknown clock allowed issuance")
		}
	}
	issuer, err := NewIssuer(reader, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading { t.Fatal("canceled call read clock"); return ClockReading{} }})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := issuer.ReadPermission(ctx); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if calls != 0 {
		t.Fatalf("read storage %d times without permission", calls)
	}
}

func TestIssuerStorageFailureDoesNotRenewReceipt(t *testing.T) {
	store := storage.NewMemory()
	generation := issuerGeneration(t, "failure-publication", 1)
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	var failure error
	reader := headReaderFunc(func(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
		if failure != nil {
			return catalogs.CatalogAuthorityHead{}, failure
		}
		return store.CurrentAuthorityHead(ctx)
	})
	issuer, err := NewIssuer(reader, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading { return ClockReading{Time: now, Known: true} }})
	if err != nil {
		t.Fatal(err)
	}
	before, err := issuer.ReadPermission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	failure = stderrors.New("current storage unavailable")
	now = now.Add(time.Minute)
	if got, err := issuer.ReadPermission(t.Context()); !stderrors.Is(err, failure) || got != (catalogs.CatalogPermissionEnvelope{}) {
		t.Fatalf("receipt=%+v error=%v", got, err)
	}
	failure = nil
	renewed, err := issuer.ReadPermission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !renewed.IssuedAt.Equal(now) || !renewed.ValidUntil.After(before.ValidUntil) {
		t.Fatal("successful fresh observation did not renew")
	}
}

func TestIssuerPreservesReceiptDuringBoundedClockCorrection(t *testing.T) {
	store := storage.NewMemory()
	generation := issuerGeneration(t, "clock-correction", 1)
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	reading := ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}
	issuer, err := NewIssuer(store, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading { return reading }})
	if err != nil {
		t.Fatal(err)
	}
	first, err := issuer.ReadPermission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	reading.Time = reading.Time.Add(time.Second)
	reading.Uncertainty = 2 * time.Second
	second, err := issuer.ReadPermission(t.Context())
	if err != nil || second != first {
		t.Fatalf("clock correction replaced the confirmed receipt: %+v %v", second, err)
	}
}

func TestIssuerRefusesRollbackAndChangedPublicationIdentity(t *testing.T) {
	for _, change := range []string{"rollback", "same sequence", "authority", "policy"} {
		t.Run(change, func(t *testing.T) {
			store := storage.NewMemory()
			first := issuerGeneration(t, "initial-publication", 1)
			if err := store.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
			issuer, err := NewIssuer(store, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading { return ClockReading{Time: now, Known: true} }})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := issuer.ReadPermission(t.Context()); err != nil {
				t.Fatal(err)
			}
			next := issuerGeneration(t, "next-publication", 2)
			switch change {
			case "same sequence":
				next.Manifest.AuthorityHead.Sequence = 1
			case "authority":
				next.Manifest.AuthorityHead.AuthorityID = "another"
			case "policy":
				next.Manifest.AuthorityHead.PolicyID = "another"
			}
			if err := store.Commit(t.Context(), next, first.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			now = now.Add(time.Minute)
			if change == "rollback" {
				if _, err := issuer.ReadPermission(t.Context()); err != nil {
					t.Fatal(err)
				}
				if err := store.Commit(t.Context(), first, next.Manifest.GenerationID); err != nil {
					t.Fatal(err)
				}
				now = now.Add(time.Minute)
			}
			if got, err := issuer.ReadPermission(t.Context()); err == nil || got != (catalogs.CatalogPermissionEnvelope{}) {
				t.Fatalf("receipt=%+v error=%v", got, err)
			}
		})
	}
}

func TestIssuerReadPermissionRejectsInvalidReceiverOrContext(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		issuer *Issuer
		ctx    context.Context
	}{
		{name: "nil receiver", ctx: t.Context()},
		{name: "zero issuer", issuer: &Issuer{}, ctx: t.Context()},
		{name: "nil context", issuer: &Issuer{}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			receipt, err := scenario.issuer.ReadPermission(scenario.ctx)
			if err == nil || receipt != (catalogs.CatalogPermissionEnvelope{}) {
				t.Fatalf("receipt=%+v error=%v", receipt, err)
			}
		})
	}
}
