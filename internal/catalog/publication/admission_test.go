package publication

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAdmissionRequiredScopeCannotBecomeOptional(t *testing.T) {
	start := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	scope := Scope{Source: sources.ModelsDevHTTPID}
	profile := Profile{Version: "test-v1", Scopes: []ScopePolicy{{Scope: scope, Required: true, Enabled: true, MaxRetainedAge: time.Hour, DisabledAction: Preserve}}}
	for _, outcome := range []AttemptOutcome{Failed, MissingCredentials, NotAttempted} {
		t.Run(string(outcome), func(t *testing.T) {
			result, err := Admit(profile, Run{StartedAt: start, CompletedAt: start.Add(time.Minute), Attempts: []Attempt{{Scope: scope, Outcome: outcome}}})
			if err != nil {
				t.Fatal(err)
			}
			if result.Allowed || len(result.Inputs) != 0 || result.Scopes[0].EvidenceKind != NoEvidence || result.Scopes[0].Attempt != outcome {
				t.Fatalf("required scope admitted without evidence: %+v", result)
			}
		})
	}
}

func TestAdmissionRetainsOriginalEvidenceWithinAgeLimit(t *testing.T) {
	start := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	scope := Scope{Source: sources.ModelsDevHTTPID}
	retained := admissionObservation(t, scope, start.Add(-time.Hour))
	profile := Profile{Version: "test-v1", Scopes: []ScopePolicy{{Scope: scope, Required: true, Enabled: true, MaxRetainedAge: time.Hour, DisabledAction: Preserve}}}
	run := Run{StartedAt: start, CompletedAt: start, Attempts: []Attempt{{Scope: scope, Outcome: Failed}}, Retained: []sources.Observation{retained}}
	result, err := Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || result.FreshAcquisition || len(result.Inputs) != 1 || result.Inputs[0].ID != retained.ID || result.Scopes[0].EvidenceKind != RetainedEvidence || !result.Scopes[0].Evidence.ObservedAt.Equal(retained.ObservedAt) {
		t.Fatalf("retained evidence changed: %+v", result)
	}
	run.CompletedAt = start.Add(time.Nanosecond)
	result, err = Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allowed || len(result.Inputs) != 0 {
		t.Fatalf("expired retained evidence accepted: %+v", result)
	}
}

func admissionObservation(t *testing.T, scope Scope, observedAt time.Time) sources.Observation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if scope.Binding != nil {
		if err := builder.SetProvider(catalogs.Provider{ID: scope.Binding.ProviderID, Name: "Test"}); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(scope.Source, catalog, sources.ObservationMetadata{ProviderBinding: scope.Binding, ObservedAt: observedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestAdmissionPartialReplyPreservesPriorCompleteEvidence(t *testing.T) {
	profile, run := admissionFixture(t)
	retained := admissionObservation(t, profile.Scopes[0].Scope, run.StartedAt.Add(-time.Minute))
	complete := *run.Attempts[0].Observation
	partial, err := sources.NewObservation(complete.SourceID, complete.Catalog, sources.ObservationMetadata{
		ProviderBinding: complete.ProviderBinding, ObservedAt: complete.ObservedAt, Revision: complete.Revision,
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeFetchFailed, Message: "private upstream error"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	run.Attempts[0].Outcome = Partial
	run.Attempts[0].Observation = &partial
	run.Retained = []sources.Observation{retained}
	decision, err := Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.FreshAcquisition || len(decision.Inputs) != 1 || decision.Inputs[0].ID != retained.ID || decision.Scopes[0].Attempt != Partial {
		t.Fatalf("partial reply replaced complete evidence: %+v", decision)
	}
	run.Retained = nil
	decision, err = Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || len(decision.Inputs) != 0 {
		t.Fatalf("partial reply admitted without prior complete evidence: %+v", decision)
	}
}

func TestAdmissionOptionalMissingRequiresProfilePermission(t *testing.T) {
	profile, run := admissionFixture(t)
	scope := Scope{Source: sources.ModelsDevHTTPID}
	profile.Scopes = append(profile.Scopes, ScopePolicy{Scope: scope, Enabled: true, DisabledAction: Preserve})
	run.Attempts = append(run.Attempts, Attempt{Scope: scope, Outcome: MissingCredentials})
	result, err := Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allowed || len(result.Inputs) != 0 {
		t.Fatalf("optional failure bypassed profile: %+v", result)
	}
	profile.Scopes[1].AllowMissing = true
	result, err = Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || !result.FreshAcquisition || len(result.Inputs) != 1 || result.Scopes[1].Evidence != nil || result.Scopes[1].Attempt != MissingCredentials {
		t.Fatalf("optional failure changed valid scope: %+v", result)
	}
}

func TestAdmissionDisabledScopeRequiresExplicitRemoval(t *testing.T) {
	for _, action := range []DisabledAction{Preserve, Remove} {
		t.Run(string(action), func(t *testing.T) {
			profile, run := admissionFixture(t)
			profile.Scopes[0].Enabled = false
			profile.Scopes[0].DisabledAction = action
			run.Attempts[0] = Attempt{Scope: profile.Scopes[0].Scope, Outcome: Disabled}
			run.Retained = []sources.Observation{admissionObservation(t, profile.Scopes[0].Scope, run.StartedAt.Add(-time.Minute))}
			result, err := Admit(profile, run)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Allowed || result.FreshAcquisition || len(result.Inputs) != 0 || result.Scopes[0].Evidence != nil || result.Scopes[0].Remove != (action == Remove) {
				t.Fatalf("disabled scope supplied current evidence: %+v", result)
			}
			if len(result.Removals) != map[DisabledAction]int{Preserve: 0, Remove: 1}[action] {
				t.Fatalf("wrong removal operations: %+v", result)
			}
		})
	}
}

func TestAdmissionDoesNotReuseOtherBindingEvidence(t *testing.T) {
	for _, field := range []string{"account", "revision", "region", "profile", "membership"} {
		t.Run(field, func(t *testing.T) {
			profile, run := admissionFixture(t)
			oldScope := profile.Scopes[0].Scope.clone()
			switch field {
			case "account":
				oldScope.Binding.AccountID = "another-account"
			case "revision":
				oldScope.Binding.Revision = "prior"
			case "region":
				oldScope.Binding.Region = "another-region"
			case "profile":
				oldScope.Binding.CredentialProfileID = "another-profile"
			case "membership":
				oldScope.Binding.MembershipAuthority = sources.ProviderMembershipEvidenceOnly
			}
			run.Retained = []sources.Observation{admissionObservation(t, oldScope, run.StartedAt.Add(-time.Minute))}
			run.Attempts[0].Outcome = Failed
			run.Attempts[0].Observation = nil
			result, err := Admit(profile, run)
			if err != nil {
				t.Fatal(err)
			}
			if result.Allowed || len(result.Inputs) != 0 {
				t.Fatalf("changed scope reused old evidence: %+v", result)
			}
		})
	}
}

func TestAdmissionEmbeddedBaselineNeedsExplicitSelection(t *testing.T) {
	profile, run := admissionFixture(t)
	embeddedScope := Scope{Source: sources.EmbeddedCatalogID}
	baseline := admissionObservation(t, embeddedScope, run.StartedAt)
	run.Retained = []sources.Observation{baseline}
	run.Attempts[0].Outcome = MissingCredentials
	run.Attempts[0].Observation = nil
	result, err := Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allowed {
		t.Fatal("embedded baseline satisfied unrelated required scope")
	}
	profile.Scopes[0].Scope = embeddedScope
	run.Attempts[0] = Attempt{Scope: embeddedScope, Outcome: Succeeded, Observation: &baseline}
	result, err = Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || result.FreshAcquisition || len(result.Inputs) != 1 {
		t.Fatalf("explicit baseline admission: %+v", result)
	}
}

func TestAdmissionReturnsOwnedMetadata(t *testing.T) {
	profile, run := admissionFixture(t)
	result, err := Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || !result.FreshAcquisition {
		t.Fatalf("complete fresh scope refused: %+v", result)
	}
	before := *profile.Scopes[0].Scope.Binding
	result.Inputs[0].ProviderBinding.AccountID = "changed-input"
	result.Scopes[0].Scope.Binding.AccountID = "changed-row"
	result.Scopes[0].Evidence.ObservationID = "changed-receipt"
	if *profile.Scopes[0].Scope.Binding != before || *run.Attempts[0].Observation.ProviderBinding != before || run.Attempts[0].Observation.ID == "changed-receipt" {
		t.Fatal("result mutation changed supplied evidence")
	}
}

func TestAdmissionRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Profile, *Run)
	}{
		{"no policy revision", func(p *Profile, _ *Run) { p.Version = "" }},
		{"unsafe revision", func(p *Profile, _ *Run) { p.Version = "test\nrevision" }},
		{"no scopes", func(p *Profile, _ *Run) { p.Scopes = nil }},
		{"duplicate policy", func(p *Profile, _ *Run) { p.Scopes = append(p.Scopes, p.Scopes[0]) }},
		{"required optional", func(p *Profile, _ *Run) { p.Scopes[0].AllowMissing = true }},
		{"negative age", func(p *Profile, _ *Run) { p.Scopes[0].MaxRetainedAge = -time.Second }},
		{"implicit removal policy", func(p *Profile, _ *Run) { p.Scopes[0].DisabledAction = "" }},
		{"unbound provider", func(p *Profile, _ *Run) { p.Scopes[0].Scope.Binding = nil }},
		{"unknown source", func(p *Profile, _ *Run) { p.Scopes[0].Scope.Source = "unknown" }},
		{"wrong source binding", func(p *Profile, _ *Run) { p.Scopes[0].Scope.Source = sources.ModelsDevHTTPID }},
		{"missing attempt", func(_ *Profile, r *Run) { r.Attempts = nil }},
		{"unknown outcome", func(_ *Profile, r *Run) { r.Attempts[0].Outcome = "unknown" }},
		{"wrong scope", func(_ *Profile, r *Run) { r.Attempts[0].Scope = Scope{Source: sources.ModelsDevHTTPID} }},
		{"disabled enabled scope", func(_ *Profile, r *Run) { r.Attempts[0].Outcome = Disabled; r.Attempts[0].Observation = nil }},
		{"attempt on disabled scope", func(p *Profile, _ *Run) { p.Scopes[0].Enabled = false }},
		{"missing direct observation", func(_ *Profile, r *Run) { r.Attempts[0].Observation = nil }},
		{"evidence with failed attempt", func(_ *Profile, r *Run) { r.Attempts[0].Outcome = Failed }},
		{"mismatched completeness", func(_ *Profile, r *Run) { r.Attempts[0].Outcome = Partial }},
		{"forged receipt", func(_ *Profile, r *Run) { r.Attempts[0].Observation.ID = "forged" }},
		{"zero run start", func(_ *Profile, r *Run) { r.StartedAt = time.Time{} }},
		{"reversed run interval", func(_ *Profile, r *Run) { r.CompletedAt = r.StartedAt.Add(-time.Second) }},
		{"fresh evidence before run", func(_ *Profile, r *Run) { r.StartedAt = r.StartedAt.Add(time.Nanosecond) }},
		{"future evidence", func(_ *Profile, r *Run) {
			r.StartedAt = r.StartedAt.Add(-time.Hour)
			r.CompletedAt = r.CompletedAt.Add(-time.Hour)
		}},
		{"future retained evidence", func(_ *Profile, r *Run) {
			r.Retained = []sources.Observation{*r.Attempts[0].Observation}
			r.StartedAt = r.StartedAt.Add(-time.Hour)
		}},
		{"duplicate retained evidence", func(_ *Profile, r *Run) {
			r.Retained = []sources.Observation{*r.Attempts[0].Observation, *r.Attempts[0].Observation}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, run := admissionFixture(t)
			tt.mutate(&profile, &run)
			if result, err := Admit(profile, run); err == nil || result.Allowed || len(result.Inputs) != 0 || len(result.Removals) != 0 {
				t.Fatalf("invalid run yielded usable decision: %+v, %v", result, err)
			}
		})
	}
}

func admissionFixture(t *testing.T) (Profile, Run) {
	t.Helper()
	start := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	scope := Scope{Source: sources.ProvidersID, Binding: &sources.ProviderAcquisitionBinding{
		SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion, ID: "provider-a", Revision: "1", ProviderID: "provider",
		AccountID: "account-a", Region: "global", APISurface: "models.list", MembershipAuthority: sources.ProviderMembershipScope,
		CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog-a",
	}}
	profile := Profile{Version: "test-v1", Scopes: []ScopePolicy{{Scope: scope, Required: true, Enabled: true, MaxRetainedAge: time.Hour, DisabledAction: Preserve}}}
	observation := admissionObservation(t, scope, start)
	run := Run{StartedAt: start, CompletedAt: start, Attempts: []Attempt{{Scope: scope.clone(), Outcome: Succeeded, Observation: &observation}}}
	return profile, run
}

func TestAdmissionZeroAgeDisablesRetainedEvidence(t *testing.T) {
	profile, run := admissionFixture(t)
	profile.Scopes[0].MaxRetainedAge = 0
	run.Retained = []sources.Observation{*run.Attempts[0].Observation}
	run.Attempts[0].Outcome = Failed
	run.Attempts[0].Observation = nil
	result, err := Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allowed || len(result.Inputs) != 0 {
		t.Fatalf("zero age reused evidence: %+v", result)
	}
}

func TestAdmissionRejectedRunCannotApplyOtherScopeRemovals(t *testing.T) {
	profile, run := admissionFixture(t)
	profile.Scopes[0].Enabled = false
	profile.Scopes[0].DisabledAction = Remove
	run.Attempts[0] = Attempt{Scope: profile.Scopes[0].Scope, Outcome: Disabled}
	scope := Scope{Source: sources.ModelsDevHTTPID}
	profile.Scopes = append(profile.Scopes, ScopePolicy{Scope: scope, Enabled: true, Required: true, DisabledAction: Preserve})
	run.Attempts = append(run.Attempts, Attempt{Scope: scope, Outcome: Failed})
	result, err := Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allowed || len(result.Inputs) != 0 || len(result.Removals) != 0 || !result.Scopes[0].Remove {
		t.Fatalf("rejected run exposed a removal operation: %+v", result)
	}
}

func TestAdmissionRejectsDuplicateAttemptsAndExcessScopes(t *testing.T) {
	profile, run := admissionFixture(t)
	scope := Scope{Source: sources.ModelsDevHTTPID}
	profile.Scopes = append(profile.Scopes, ScopePolicy{Scope: scope, Enabled: true, Required: true, DisabledAction: Preserve})
	run.Attempts = append(run.Attempts, run.Attempts[0])
	if _, err := Admit(profile, run); err == nil {
		t.Fatal("duplicate attempts concealed missing scope")
	}
	profile, run = admissionFixture(t)
	profile.Scopes = make([]ScopePolicy, maxScopes+1)
	if _, err := Admit(profile, run); err == nil {
		t.Fatal("unbounded profile accepted")
	}
	profile, run = admissionFixture(t)
	run.Retained = make([]sources.Observation, maxScopes+1)
	if _, err := Admit(profile, run); err == nil {
		t.Fatal("unbounded retained evidence accepted")
	}
}

func TestAdmissionRejectsInvalidRetainedReceipt(t *testing.T) {
	profile, run := admissionFixture(t)
	observation := *run.Attempts[0].Observation
	observation.EvidenceChecksum = "changed"
	run.Retained = []sources.Observation{observation}
	if _, err := Admit(profile, run); err == nil {
		t.Fatal("retained checksum mismatch accepted")
	}
}
