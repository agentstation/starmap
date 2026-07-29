package catalogs

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/agentstation/starmap/pkg/errors"
)

// AuthoredModel is one provider-independent construction record stored at
// authors/<author>/models/<slug>.yaml. Model contains intrinsic facts only.
type AuthoredModel struct {
	AuthorID AuthorID
	Model    Model
}

// ID returns the canonical author/slug identity.
func (m AuthoredModel) ID() ModelDefinitionID {
	return AuthoredModelID(m.AuthorID, m.Model.ID)
}

func copyAuthoredModel(record AuthoredModel) AuthoredModel {
	record.Model = DeepCopyModel(record.Model)
	return record
}

type authoredModelStore struct {
	mu      sync.RWMutex
	records map[ModelDefinitionID]AuthoredModel
}

func newAuthoredModelStore() *authoredModelStore {
	return &authoredModelStore{records: make(map[ModelDefinitionID]AuthoredModel)}
}

func (s *authoredModelStore) set(record AuthoredModel) error {
	if err := validateAuthoredModel(record.AuthorID, record.Model); err != nil {
		return err
	}
	s.mu.Lock()
	s.records[record.ID()] = copyAuthoredModel(record)
	s.mu.Unlock()
	return nil
}

func (s *authoredModelStore) delete(id ModelDefinitionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, found := s.records[id]; !found {
		return &errors.NotFoundError{Resource: "authored model", ID: string(id)}
	}
	delete(s.records, id)
	return nil
}

func (s *authoredModelStore) list() []AuthoredModel {
	s.mu.RLock()
	result := make([]AuthoredModel, 0, len(s.records))
	for _, record := range s.records {
		result = append(result, copyAuthoredModel(record))
	}
	s.mu.RUnlock()
	slices.SortFunc(result, func(left, right AuthoredModel) int {
		return strings.Compare(string(left.ID()), string(right.ID()))
	})
	return result
}

func (s *authoredModelStore) clear() {
	s.mu.Lock()
	clear(s.records)
	s.mu.Unlock()
}

// AuthoredModelID returns the canonical author/slug identity for one authored
// model record.
func AuthoredModelID(authorID AuthorID, slug string) ModelDefinitionID {
	return ModelDefinitionID(string(authorID) + "/" + slug)
}

// ParseModelDefinitionID validates and splits one canonical author/slug ID.
func ParseModelDefinitionID(id ModelDefinitionID) (AuthorID, string, error) {
	raw := string(id)
	if strings.Count(raw, "/") != 1 {
		return "", "", &errors.ValidationError{
			Field: "model", Value: id,
			Message: "must contain exactly one author/slug separator",
		}
	}
	author, slug, _ := strings.Cut(raw, "/")
	if err := validatePathSegment("model.author", author); err != nil {
		return "", "", err
	}
	if err := validatePathSegment("model.slug", slug); err != nil {
		return "", "", err
	}
	return AuthorID(author), slug, nil
}

// validateAuthoredModel enforces the disjoint ownership contract between an
// authored model and a provider serving record.
func validateAuthoredModel(authorID AuthorID, model Model) error {
	if err := validatePathSegment("author_id", string(authorID)); err != nil {
		return err
	}
	if err := validatePathSegment("authored_model.id", model.ID); err != nil {
		return err
	}
	if model.ModelRef != "" {
		return &errors.ValidationError{
			Field: "authored_model.model", Value: model.ModelRef,
			Message: "must be empty because the containing author and slug are canonical identity",
		}
	}
	if strings.TrimSpace(model.Name) == "" {
		return &errors.ValidationError{Field: "authored_model.name", Message: "is required"}
	}
	if err := validateModelGeneration(model.Generation); err != nil {
		return err
	}
	if len(model.Authors) == 0 || model.Authors[0].ID != authorID {
		return &errors.ValidationError{
			Field: "authored_model.authors", Value: model.Authors,
			Message: "primary author must match the containing author",
		}
	}
	if model.Status != "" || model.Pricing != nil || model.Limits != nil || len(model.Modes) != 0 {
		return &errors.ValidationError{
			Field:   "authored_model",
			Message: "must not contain provider status, pricing, limits, or modes",
		}
	}
	for source := range model.Extensions {
		if source != "models.dev" {
			return &errors.ValidationError{
				Field: "authored_model.extensions", Value: source,
				Message: "must not contain provider-specific extensions",
			}
		}
	}
	return nil
}

func validateModelGeneration(generation *ModelGeneration) error {
	if generation == nil {
		return nil
	}
	floatRanges := []struct {
		name  string
		value *FloatRange
	}{
		{"temperature", generation.Temperature},
		{"top_p", generation.TopP},
		{"top_a", generation.TopA},
		{"min_p", generation.MinP},
		{"typical_p", generation.TypicalP},
		{"tfs", generation.TFS},
		{"frequency_penalty", generation.FrequencyPenalty},
		{"presence_penalty", generation.PresencePenalty},
		{"repetition_penalty", generation.RepetitionPenalty},
		{"length_penalty", generation.LengthPenalty},
		{"mirostat_tau", generation.MirostatTau},
		{"mirostat_eta", generation.MirostatEta},
		{"contrastive_search_penalty_alpha", generation.ContrastiveSearchPenaltyAlpha},
		{"diversity_penalty", generation.DiversityPenalty},
	}
	for _, candidate := range floatRanges {
		if candidate.value == nil {
			continue
		}
		if candidate.value.Min > candidate.value.Max ||
			candidate.value.Default < candidate.value.Min ||
			candidate.value.Default > candidate.value.Max {
			return &errors.ValidationError{
				Field: "model.generation." + candidate.name,
				Value: *candidate.value,
				Message: fmt.Sprintf(
					"default must be within the inclusive range [%v, %v]",
					candidate.value.Min,
					candidate.value.Max,
				),
			}
		}
	}
	intRanges := []struct {
		name  string
		value *IntRange
	}{
		{"top_k", generation.TopK},
		{"no_repeat_ngram_size", generation.NoRepeatNgramSize},
		{"n", generation.N},
		{"best_of", generation.BestOf},
		{"num_beams", generation.NumBeams},
	}
	for _, candidate := range intRanges {
		if candidate.value == nil {
			continue
		}
		if candidate.value.Min > candidate.value.Max ||
			candidate.value.Default < candidate.value.Min ||
			candidate.value.Default > candidate.value.Max {
			return &errors.ValidationError{
				Field: "model.generation." + candidate.name,
				Value: *candidate.value,
				Message: fmt.Sprintf(
					"default must be within the inclusive range [%d, %d]",
					candidate.value.Min,
					candidate.value.Max,
				),
			}
		}
	}
	return nil
}

func validatePathSegment(field, value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return &errors.ValidationError{
			Field: field, Value: value,
			Message: "must be a non-empty canonical path segment",
		}
	}
	if value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
		return &errors.ValidationError{
			Field: field, Value: value,
			Message: "must not contain path traversal or separators",
		}
	}
	return nil
}

func validateProviderModelPathID(id string) error {
	if id == "" || strings.TrimSpace(id) != id || strings.Contains(id, `\`) {
		return &errors.ValidationError{
			Field: "provider_model.id", Value: id,
			Message: "must be a non-empty relative slash-separated ID",
		}
	}
	for _, segment := range strings.Split(id, "/") {
		if err := validatePathSegment("provider_model.id", segment); err != nil {
			return err
		}
	}
	return nil
}
