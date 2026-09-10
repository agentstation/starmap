package runtime

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/sources"
)

type passiveSourceCollector struct {
	t             *testing.T
	configuration []sources.SourceActivity
	declarations  int
}

func (c *passiveSourceCollector) AcquireSources(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
	c.t.Error("passive status acquired sources")
	return nil, context.Canceled
}
func (c *passiveSourceCollector) SourceConfiguration() []sources.SourceActivity {
	c.declarations++
	return c.configuration
}

func TestRuntimeSourceConfigurationIsPassiveAndOwned(t *testing.T) {
	collector := &passiveSourceCollector{t: t, configuration: []sources.SourceActivity{{Source: sources.LocalCatalogID, Supported: true, Enabled: true, Eligibility: sources.EligibilityUnknown}}}
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithSourceAcquirer(collector), WithAcquisitionEnabled(false))
	collector.configuration[0].Source = sources.ModelsDevGitID
	for range 2 {
		status := connected.Status()
		if len(status.SourceConfiguration) != 4 {
			t.Fatalf("source configuration=%+v", status.SourceConfiguration)
		}
		var found bool
		for _, row := range status.SourceConfiguration {
			if row.Eligibility != sources.EligibilityUnknown || row.Attempted {
				t.Fatalf("passive status claims acquisition: %+v", row)
			}
			if row.Source == sources.LocalCatalogID {
				found = row.Supported && row.Enabled && !row.SupportUnknown && !row.SelectionUnknown
			}
		}
		if !found {
			t.Fatalf("local configuration=%+v", status.SourceConfiguration)
		}
		status.SourceConfiguration[0].Source = "caller-change"
	}
	if collector.declarations != 1 {
		t.Fatalf("declarations=%d, want one constructor snapshot", collector.declarations)
	}
}

func TestSourceConfigurationPreservesUnknownCapabilities(t *testing.T) {
	opaque := sourceAcquirerFunc(func(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error) {
		t.Fatal("configuration acquired a source")
		return nil, nil
	})
	for _, explicit := range []bool{false, true} {
		config := &options{sourceAcquirer: opaque}
		if explicit {
			config.acquisitionSources = &acquisitionSourcePolicy{ids: []sources.ID{sources.ModelsDevHTTPID}}
		}
		rows, err := describeSources(config)
		if err != nil || len(rows) != 4 {
			t.Fatalf("configuration=%+v, error=%v", rows, err)
		}
		for _, row := range rows {
			if !row.Valid() {
				t.Fatalf("invalid state=%+v", row)
			}
			if row.Source == sources.ProvidersID {
				if row.Supported || row.SupportUnknown {
					t.Fatalf("missing provider role=%+v", row)
				}
				continue
			}
			if !row.SupportUnknown || row.Supported || row.SelectionUnknown == explicit {
				t.Fatalf("unknown source=%+v", row)
			}
			if row.Enabled != (explicit && row.Source == sources.ModelsDevHTTPID) {
				t.Fatalf("source selection=%+v", row)
			}
		}
	}
}

func TestSourceConfigurationRejectsAcquisitionClaims(t *testing.T) {
	for _, test := range []struct {
		name string
		rows []sources.SourceActivity
	}{
		{"attempted", []sources.SourceActivity{{Source: sources.LocalCatalogID, Supported: true, Enabled: true, Attempted: true, Eligibility: sources.EligibilityUnknown}}},
		{"eligible", []sources.SourceActivity{{Source: sources.LocalCatalogID, Supported: true, Enabled: true, Eligibility: sources.EligibilityEligible}}},
		{"provider", []sources.SourceActivity{{Source: sources.ProvidersID, Eligibility: sources.EligibilityUnknown}}},
		{"duplicate", []sources.SourceActivity{{Source: sources.LocalCatalogID, Eligibility: sources.EligibilityUnknown}, {Source: sources.LocalCatalogID, Eligibility: sources.EligibilityUnknown}}},
		{"unknown-source", []sources.SourceActivity{{Source: "private", Eligibility: sources.EligibilityUnknown}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			collector := &passiveSourceCollector{t: t, configuration: test.rows}
			if _, err := describeSources(&options{sourceAcquirer: collector}); err == nil {
				t.Fatal("invalid passive declaration succeeded")
			}
		})
	}
}
