package publication

import (
	"bytes"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublicationAuthoredEditDoesNotRetainAcquisitionFields(t *testing.T) {
	catalog, profile := producerPipelineFixture(t)
	state, err := NewState(publicationBaselineFixture(t, catalog), "publisher")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	run := Run{StartedAt: at, CompletedAt: at}
	for _, policy := range profile.Scopes {
		attempt := Attempt{Scope: policy.Scope, Outcome: Disabled}
		if policy.Enabled {
			observation := publicationInventoryAt(t, catalog, policy.Scope.Binding, 6, at)
			attempt.Outcome, attempt.Observation = Succeeded, &observation
		}
		run.Attempts = append(run.Attempts, attempt)
	}
	first, err := preparePublication(t.Context(), state, profile, run, "provider-update")
	if err != nil {
		t.Fatal(err)
	}
	original, err := EncodeState(first.Next)
	if err != nil {
		t.Fatal(err)
	}
	for index := range profile.Scopes {
		profile.Scopes[index].Enabled = false
		profile.Scopes[index].DisabledAction = Remove
	}
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	promoted, err := catalogs.DecodeCatalogGeneration(first.Next.Generation())
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"provider-name", "offering-name", "nested-limit", "authored-description", "provider-config-removal", "offering-removal", "offering-addition"} {
		t.Run(change, func(t *testing.T) {
			edited, err := catalogs.NewBuilderFrom(promoted)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := edited.Provider("provider")
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "provider-name":
				provider.Name = "Edited provider"
			case "offering-name":
				provider.Models["one"].Name = "Edited offering"
			case "nested-limit":
				provider.Models["one"].Limits.OutputTokens = 128
			case "provider-config-removal":
				provider.Catalog = nil
			case "offering-removal":
				delete(provider.Models, "two")
			case "offering-addition":
				provider.Models["authored"] = &catalogs.Model{ID: "authored", Name: "Authored offering", ModelRef: "author/one"}
			case "authored-description":
				for _, record := range promoted.AuthoredModels() {
					if record.AuthorID == "author" && record.Model.ID == "one" {
						record.Model.Description = "Authored description"
						if err := edited.SetAuthorModel("author", record.Model); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			if err := edited.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			authored, err := edited.Build()
			if err != nil {
				t.Fatal(err)
			}
			replacement := first.Next.Generation()
			replacement.Payload, err = catalogs.EncodeCatalogPayload(authored)
			if err != nil {
				t.Fatal(err)
			}
			replacement.Manifest.Payload = catalogs.DescribeCatalogPayload(replacement.Payload)
			replacement.Manifest.GenerationID = "authored-" + change
			next, err := producer.PrepareWithBaseline(t.Context(), first.Next, replacement, "authored-"+change)
			if err != nil {
				t.Fatal(err)
			}
			checkpoint, err := EncodeState(next.Next)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
			if err != nil {
				t.Fatal(err)
			}
			after, err := catalogs.DecodeCatalogGeneration(restored.Generation())
			if err != nil {
				t.Fatal(err)
			}
			provider, err = after.Provider("provider")
			if err != nil {
				t.Fatal(err)
			}
			if limits := provider.Models["one"].Limits; limits != nil && limits.ContextWindow == 200 {
				t.Error("unrelated authored edit made acquired context limit permanent")
			}
			if len(restored.history) != 0 || len(after.MembershipScopes()) != 0 {
				t.Error("unrelated authored edit made acquired scope state permanent")
			}
			switch change {
			case "provider-name":
				if provider.Name != "Edited provider" {
					t.Error("lost the authored provider name")
				}
			case "offering-name":
				if provider.Models["one"].Name != "Edited offering" {
					t.Error("lost the authored offering name")
				}
			case "nested-limit":
				if provider.Models["one"].Limits == nil || provider.Models["one"].Limits.OutputTokens != 128 {
					t.Error("lost the authored output limit")
				}
			case "provider-config-removal":
				if provider.Catalog != nil {
					t.Error("restored the explicitly removed provider configuration")
				}
			case "offering-removal":
				if provider.Models["two"] != nil {
					t.Error("restored the explicitly removed offering")
				}
			case "offering-addition":
				model := provider.Models["authored"]
				if model == nil || model.ModelRef != "author/one" || model.Name != "Authored offering" {
					t.Error("lost the added offering or its exact reference")
				}
			case "authored-description":
				model, err := after.AuthorModel("author", "one")
				if err != nil || model.Description != "Authored description" {
					t.Error("lost the authored description", err)
				}
			}
			unchanged, err := EncodeState(first.Next)
			if err != nil || !bytes.Equal(original.Data, unchanged.Data) {
				t.Fatal("authored preparation changed the accepted checkpoint", err)
			}
		})
	}
}

func TestPublicationAuthoredOfferingMaterializesOnlyRequiredIdentity(t *testing.T) {
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewState(publicationBaselineFixture(t, empty), "publisher")
	if err != nil {
		t.Fatal(err)
	}
	observed, _ := producerPipelineFixture(t)
	builder, err := catalogs.NewBuilderFrom(observed)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models["one"].Limits = &catalogs.ModelLimits{ContextWindow: 200}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	observed, err = builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	policy := ScopePolicy{Scope: Scope{Source: sources.ModelsDevHTTPID}, Required: true, Enabled: true, DisabledAction: Preserve}
	profile := Profile{Version: "authored-source-only", Scopes: []ScopePolicy{policy}}
	observation, err := sources.NewObservation(policy.Scope.Source, observed, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := preparePublication(t.Context(), state, profile, Run{StartedAt: at, CompletedAt: at,
		Attempts: []Attempt{{Scope: policy.Scope, Outcome: Succeeded, Observation: &observation}}}, "metadata-first")
	if err != nil {
		t.Fatal(err)
	}
	promoted, err := catalogs.DecodeCatalogGeneration(first.Next.Generation())
	if err != nil {
		t.Fatal(err)
	}
	builder, err = catalogs.NewBuilderFrom(promoted)
	if err != nil {
		t.Fatal(err)
	}
	provider, err = builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Models["one"].Limits == nil || provider.Models["one"].Limits.ContextWindow != 200 {
		t.Fatal("fixture did not acquire the offering limit")
	}
	provider.Models["one"].Limits.OutputTokens = 9007199254740993
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	edited, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	replacement := first.Next.Generation()
	replacement.Payload, err = catalogs.EncodeCatalogPayload(edited)
	if err != nil {
		t.Fatal(err)
	}
	replacement.Manifest.Payload = catalogs.DescribeCatalogPayload(replacement.Payload)
	profile.Scopes[0].Enabled = false
	profile.Scopes[0].DisabledAction = Remove
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := producer.PrepareWithBaseline(t.Context(), first.Next, replacement, "authored-only")
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := EncodeState(next.Next)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	after, err := catalogs.DecodeCatalogGeneration(restored.Generation())
	if err != nil {
		t.Fatal(err)
	}
	provider, err = after.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.Models) != 1 || provider.Models["one"] == nil {
		t.Fatal("authored edit retained unrelated acquired offerings")
	}
	model := provider.Models["one"]
	if model.ModelRef != "author/one" || model.Limits == nil || model.Limits.OutputTokens != 9007199254740993 || model.Limits.ContextWindow != 0 {
		t.Fatal("authored edit lost exact identity or numeric value, or retained unrelated acquired limits")
	}
	if provider.Catalog != nil || provider.Credentials != nil || len(after.AuthoredModels()) != 1 || len(after.Authors().List()) != 1 {
		t.Fatal("authored edit retained unrelated acquired configuration or definitions")
	}
	if len(restored.history) != 0 || len(after.MembershipScopes()) != 0 {
		t.Fatal("authored edit retained removed acquisition state")
	}
}

func TestPublicationAuthoredEditDoesNotRetainRemovedSourceReviews(t *testing.T) {
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewState(publicationBaselineFixture(t, empty), "publisher")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	policy := ScopePolicy{Scope: Scope{Source: sources.ModelsDevHTTPID}, Required: true, Enabled: true, DisabledAction: Preserve}
	profile := Profile{Version: "authored-review", Scopes: []ScopePolicy{policy}}
	observation := unresolvedPublicationObservation(t, at)
	first, err := preparePublication(t.Context(), state, profile, Run{StartedAt: at, CompletedAt: at,
		Attempts: []Attempt{{Scope: policy.Scope, Outcome: Succeeded, Observation: &observation}}}, "unresolved-source")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Next.Generation().Manifest.ReviewCandidates) != 1 {
		t.Fatal("fixture did not publish the source review")
	}
	promoted, err := catalogs.DecodeCatalogGeneration(first.Next.Generation())
	if err != nil {
		t.Fatal(err)
	}
	builder, err := catalogs.NewBuilderFrom(promoted)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Name = "Authored provider"
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	edited, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	replacement := first.Next.Generation()
	replacement.Payload, err = catalogs.EncodeCatalogPayload(edited)
	if err != nil {
		t.Fatal(err)
	}
	replacement.Manifest.Payload = catalogs.DescribeCatalogPayload(replacement.Payload)
	profile.Scopes[0].Enabled = false
	profile.Scopes[0].DisabledAction = Remove
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := producer.PrepareWithBaseline(t.Context(), first.Next, replacement, "authored-remove-source")
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := artifact.DecodePublicationReceipt(next.Receipt.Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Reviews) != 0 || len(next.Next.Generation().Manifest.ReviewCandidates) != 0 {
		t.Fatal("unrelated authored edit made an acquired review permanent")
	}
}
