package reconcile

import (
	"fmt"
	"reflect"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ThreeWayMerger performs three-way merges with conflict resolution
type ThreeWayMerger interface {
	// MergeModels performs a three-way merge on models
	MergeModels(base, ours, theirs catalogs.Model) (catalogs.Model, []Conflict, error)
	
	// MergeProviders performs a three-way merge on providers
	MergeProviders(base, ours, theirs catalogs.Provider) (catalogs.Provider, []Conflict, error)
	
	// MergeAuthors performs a three-way merge on authors
	MergeAuthors(base, ours, theirs catalogs.Author) (catalogs.Author, []Conflict, error)
	
	// ResolveConflicts applies a conflict resolution strategy
	ResolveConflicts(conflicts []Conflict, strategy ConflictResolution) []Resolution
}

// Conflict represents a merge conflict
type Conflict struct {
	Path      string      // Field path (e.g., "pricing.input")
	Base      interface{} // Original value
	Ours      interface{} // Our change
	Theirs    interface{} // Their change
	Type      ConflictType
	CanMerge  bool        // Whether automatic merge is possible
	Suggested interface{} // Suggested resolution
}

// ConflictType describes the type of conflict
type ConflictType string

const (
	ConflictTypeModified ConflictType = "modified"  // Both sides modified the same field
	ConflictTypeDeleted  ConflictType = "deleted"   // One side deleted, other modified
	ConflictTypeAdded    ConflictType = "added"     // Both sides added different values
)

// ConflictResolution defines how to resolve conflicts
type ConflictResolution string

const (
	ResolutionOurs      ConflictResolution = "ours"      // Always take our changes
	ResolutionTheirs    ConflictResolution = "theirs"    // Always take their changes
	ResolutionMerge     ConflictResolution = "merge"     // Try to merge if possible
	ResolutionBase      ConflictResolution = "base"      // Keep original value
	ResolutionNewest    ConflictResolution = "newest"    // Take the newest change
	ResolutionAuthority ConflictResolution = "authority" // Use field authorities
)

// Resolution represents a conflict resolution decision
type Resolution struct {
	Conflict Conflict
	Decision ConflictResolution
	Value    interface{}
	Reason   string
}

// threeWayMerger is the default implementation
type threeWayMerger struct {
	authorities AuthorityProvider
	strategy    Strategy
	tracker     ProvenanceTracker
}

// NewThreeWayMerger creates a new three-way merger
func NewThreeWayMerger(authorities AuthorityProvider, strategy Strategy) ThreeWayMerger {
	return &threeWayMerger{
		authorities: authorities,
		strategy:    strategy,
	}
}

// WithProvenance enables provenance tracking
func (m *threeWayMerger) WithProvenance(tracker ProvenanceTracker) *threeWayMerger {
	m.tracker = tracker
	return m
}

// MergeModels performs a three-way merge on models
func (m *threeWayMerger) MergeModels(base, ours, theirs catalogs.Model) (catalogs.Model, []Conflict, error) {
	merged := ours // Start with our version
	conflicts := []Conflict{}
	
	// Compare each field to detect conflicts and merge
	
	// Name
	if nameConflict := m.detectConflict("name", base.Name, ours.Name, theirs.Name); nameConflict != nil {
		conflicts = append(conflicts, *nameConflict)
		// Apply automatic resolution if possible
		if nameConflict.CanMerge {
			merged.Name = nameConflict.Suggested.(string)
		}
	} else if theirs.Name != base.Name {
		merged.Name = theirs.Name // No conflict, take their change
	}
	
	// Description
	if descConflict := m.detectConflict("description", base.Description, ours.Description, theirs.Description); descConflict != nil {
		conflicts = append(conflicts, *descConflict)
		if descConflict.CanMerge {
			merged.Description = descConflict.Suggested.(string)
		}
	} else if theirs.Description != base.Description {
		merged.Description = theirs.Description
	}
	
	// Pricing - complex nested structure
	pricingMerged, pricingConflicts := m.mergePricing(base.Pricing, ours.Pricing, theirs.Pricing)
	if pricingMerged != nil {
		merged.Pricing = pricingMerged
	}
	conflicts = append(conflicts, pricingConflicts...)
	
	// Limits
	limitsMerged, limitsConflicts := m.mergeLimits(base.Limits, ours.Limits, theirs.Limits)
	if limitsMerged != nil {
		merged.Limits = limitsMerged
	}
	conflicts = append(conflicts, limitsConflicts...)
	
	// Features
	featuresMerged, featuresConflicts := m.mergeFeatures(base.Features, ours.Features, theirs.Features)
	if featuresMerged != nil {
		merged.Features = featuresMerged
	}
	conflicts = append(conflicts, featuresConflicts...)
	
	// Metadata
	metadataMerged, metadataConflicts := m.mergeMetadata(base.Metadata, ours.Metadata, theirs.Metadata)
	if metadataMerged != nil {
		merged.Metadata = metadataMerged
	}
	conflicts = append(conflicts, metadataConflicts...)
	
	// Track provenance if enabled
	if m.tracker != nil {
		m.trackMerge(merged, base, ours, theirs)
	}
	
	return merged, conflicts, nil
}

// detectConflict detects if there's a conflict between values
func (m *threeWayMerger) detectConflict(path string, base, ours, theirs interface{}) *Conflict {
	// No conflict if both made the same change
	if reflect.DeepEqual(ours, theirs) {
		return nil
	}
	
	// No conflict if only one side changed
	oursChanged := !reflect.DeepEqual(base, ours)
	theirsChanged := !reflect.DeepEqual(base, theirs)
	
	if !oursChanged {
		return nil // We didn't change, take theirs
	}
	if !theirsChanged {
		return nil // They didn't change, keep ours
	}
	
	// Both changed to different values - conflict!
	conflict := &Conflict{
		Path:   path,
		Base:   base,
		Ours:   ours,
		Theirs: theirs,
		Type:   ConflictTypeModified,
	}
	
	// Try to determine if automatic merge is possible
	conflict.CanMerge, conflict.Suggested = m.suggestResolution(path, base, ours, theirs)
	
	return conflict
}

// suggestResolution suggests a resolution for a conflict
func (m *threeWayMerger) suggestResolution(path string, base, ours, theirs interface{}) (bool, interface{}) {
	// Use field authorities if available
	if m.authorities != nil {
		authority := m.authorities.GetAuthority(path, ResourceTypeModel)
		if authority != nil {
			// For now, don't auto-merge based on authorities - let the user decide
			// In practice, we'd need source information to determine which wins
			return false, nil
		}
	}
	
	// Try strategy-based resolution
	if m.strategy != nil {
		values := map[SourceName]interface{}{
			"base":   base,
			"ours":   ours,
			"theirs": theirs,
		}
		value, _, _ := m.strategy.ResolveConflict(path, values)
		if value != nil {
			return true, value
		}
	}
	
	// Type-specific merging strategies
	switch v := ours.(type) {
	case string:
		// For strings, could try to merge if they're concatenatable
		if base == "" && ours != "" && theirs != "" {
			// Both added content to empty field
			return true, fmt.Sprintf("%v\n%v", ours, theirs)
		}
	case int, int64, float64:
		// For numbers, could take the average or max/min
		// This is domain-specific logic
		return false, nil
	case bool:
		// For booleans, no automatic merge possible
		return false, nil
	default:
		_ = v
	}
	
	return false, nil
}

// mergePricing merges pricing structures
func (m *threeWayMerger) mergePricing(base, ours, theirs *catalogs.ModelPricing) (*catalogs.ModelPricing, []Conflict) {
	conflicts := []Conflict{}
	
	// Handle nil cases
	if ours == nil && theirs == nil {
		return nil, conflicts
	}
	if ours == nil {
		return theirs, conflicts
	}
	if theirs == nil {
		return ours, conflicts
	}
	
	merged := &catalogs.ModelPricing{}
	*merged = *ours // Start with our version
	
	// Merge token pricing
	if base == nil {
		base = &catalogs.ModelPricing{}
	}
	
	if ours.Tokens != nil || theirs.Tokens != nil {
		var baseTokens *catalogs.TokenPricing
		if base.Tokens != nil {
			baseTokens = base.Tokens
		}
		
		mergedTokens, tokenConflicts := m.mergeTokenPricing(baseTokens, ours.Tokens, theirs.Tokens)
		merged.Tokens = mergedTokens
		conflicts = append(conflicts, tokenConflicts...)
	}
	
	// Merge operation pricing
	if ours.Operations != nil || theirs.Operations != nil {
		// Similar logic for operations
		merged.Operations = ours.Operations
		if theirs.Operations != nil && ours.Operations == nil {
			merged.Operations = theirs.Operations
		}
	}
	
	// Currency
	if conflict := m.detectConflict("pricing.currency", base.Currency, ours.Currency, theirs.Currency); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if theirs.Currency != base.Currency {
		merged.Currency = theirs.Currency
	}
	
	return merged, conflicts
}

// mergeTokenPricing merges token pricing structures
func (m *threeWayMerger) mergeTokenPricing(base, ours, theirs *catalogs.TokenPricing) (*catalogs.TokenPricing, []Conflict) {
	conflicts := []Conflict{}
	
	// Handle nil cases
	if ours == nil && theirs == nil {
		return nil, conflicts
	}
	if ours == nil {
		return theirs, conflicts
	}
	if theirs == nil {
		return ours, conflicts
	}
	
	merged := &catalogs.TokenPricing{}
	*merged = *ours // Start with our version
	
	if base == nil {
		base = &catalogs.TokenPricing{}
	}
	
	// Input pricing
	if ours.Input != nil || theirs.Input != nil {
		var baseInput *catalogs.TokenCost
		if base.Input != nil {
			baseInput = base.Input
		}
		
		if conflict := m.detectTokenCostConflict("pricing.tokens.input", baseInput, ours.Input, theirs.Input); conflict != nil {
			conflicts = append(conflicts, *conflict)
		} else if theirs.Input != nil && !tokenCostEqual(baseInput, theirs.Input) {
			merged.Input = theirs.Input
		}
	}
	
	// Output pricing
	if ours.Output != nil || theirs.Output != nil {
		var baseOutput *catalogs.TokenCost
		if base.Output != nil {
			baseOutput = base.Output
		}
		
		if conflict := m.detectTokenCostConflict("pricing.tokens.output", baseOutput, ours.Output, theirs.Output); conflict != nil {
			conflicts = append(conflicts, *conflict)
		} else if theirs.Output != nil && !tokenCostEqual(baseOutput, theirs.Output) {
			merged.Output = theirs.Output
		}
	}
	
	return merged, conflicts
}

// detectTokenCostConflict detects conflicts in token cost
func (m *threeWayMerger) detectTokenCostConflict(path string, base, ours, theirs *catalogs.TokenCost) *Conflict {
	// Convert to interface{} for comparison
	var baseVal, oursVal, theirsVal interface{}
	if base != nil {
		baseVal = base.Per1M
	}
	if ours != nil {
		oursVal = ours.Per1M
	}
	if theirs != nil {
		theirsVal = theirs.Per1M
	}
	
	return m.detectConflict(path, baseVal, oursVal, theirsVal)
}

// tokenCostEqual compares two TokenCost pointers
func tokenCostEqual(a, b *catalogs.TokenCost) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Per1M == b.Per1M && a.PerToken == b.PerToken
}

// mergeLimits merges model limits
func (m *threeWayMerger) mergeLimits(base, ours, theirs *catalogs.ModelLimits) (*catalogs.ModelLimits, []Conflict) {
	conflicts := []Conflict{}
	
	// Handle nil cases
	if ours == nil && theirs == nil {
		return nil, conflicts
	}
	if ours == nil {
		return theirs, conflicts
	}
	if theirs == nil {
		return ours, conflicts
	}
	
	merged := &catalogs.ModelLimits{}
	*merged = *ours // Start with our version
	
	if base == nil {
		base = &catalogs.ModelLimits{}
	}
	
	// Context window
	if conflict := m.detectConflict("limits.context_window", base.ContextWindow, ours.ContextWindow, theirs.ContextWindow); conflict != nil {
		// For limits, we want to take the maximum value
		if ours.ContextWindow > 0 && theirs.ContextWindow > 0 {
			maxValue := ours.ContextWindow
			if theirs.ContextWindow > ours.ContextWindow {
				maxValue = theirs.ContextWindow
			}
			merged.ContextWindow = maxValue
			conflict.CanMerge = true
			conflict.Suggested = int64(maxValue)
		}
		conflicts = append(conflicts, *conflict)
	} else if theirs.ContextWindow != base.ContextWindow {
		merged.ContextWindow = theirs.ContextWindow
	}
	
	// Output tokens
	if conflict := m.detectConflict("limits.output_tokens", base.OutputTokens, ours.OutputTokens, theirs.OutputTokens); conflict != nil {
		// Take maximum for output tokens as well
		if ours.OutputTokens > 0 && theirs.OutputTokens > 0 {
			maxValue := ours.OutputTokens
			if theirs.OutputTokens > ours.OutputTokens {
				maxValue = theirs.OutputTokens
			}
			merged.OutputTokens = maxValue
			conflict.CanMerge = true
			conflict.Suggested = int64(maxValue)
		}
		conflicts = append(conflicts, *conflict)
	} else if theirs.OutputTokens != base.OutputTokens {
		merged.OutputTokens = theirs.OutputTokens
	}
	
	return merged, conflicts
}

// mergeFeatures merges model features
func (m *threeWayMerger) mergeFeatures(base, ours, theirs *catalogs.ModelFeatures) (*catalogs.ModelFeatures, []Conflict) {
	conflicts := []Conflict{}
	
	// Handle nil cases
	if ours == nil && theirs == nil {
		return nil, conflicts
	}
	if ours == nil {
		return theirs, conflicts
	}
	if theirs == nil {
		return ours, conflicts
	}
	
	merged := &catalogs.ModelFeatures{}
	*merged = *ours // Start with our version
	
	if base == nil {
		base = &catalogs.ModelFeatures{}
	}
	
	// Merge modalities (complex structure)
	if modalityConflict := m.detectModalityConflict("features.modalities", base.Modalities, ours.Modalities, theirs.Modalities); modalityConflict != nil {
		// For modalities, we could union them
		merged.Modalities = unionModalities(ours.Modalities, theirs.Modalities)
		modalityConflict.CanMerge = true
		modalityConflict.Suggested = merged.Modalities
		conflicts = append(conflicts, *modalityConflict)
	} else if !modalitiesEqual(base.Modalities, theirs.Modalities) {
		merged.Modalities = theirs.Modalities
	}
	
	// Boolean features - for these, "true" typically wins (capability added)
	boolFields := map[string]*bool{
		"features.tool_calls":  &merged.ToolCalls,
		"features.tools":       &merged.Tools,
		"features.tool_choice": &merged.ToolChoice,
		"features.web_search":  &merged.WebSearch,
		"features.reasoning":   &merged.Reasoning,
	}
	
	baseBools := map[string]bool{
		"features.tool_calls":  base.ToolCalls,
		"features.tools":       base.Tools,
		"features.tool_choice": base.ToolChoice,
		"features.web_search":  base.WebSearch,
		"features.reasoning":   base.Reasoning,
	}
	
	oursBools := map[string]bool{
		"features.tool_calls":  ours.ToolCalls,
		"features.tools":       ours.Tools,
		"features.tool_choice": ours.ToolChoice,
		"features.web_search":  ours.WebSearch,
		"features.reasoning":   ours.Reasoning,
	}
	
	theirsBools := map[string]bool{
		"features.tool_calls":  theirs.ToolCalls,
		"features.tools":       theirs.Tools,
		"features.tool_choice": theirs.ToolChoice,
		"features.web_search":  theirs.WebSearch,
		"features.reasoning":   theirs.Reasoning,
	}
	
	for field, mergedField := range boolFields {
		baseVal := baseBools[field]
		oursVal := oursBools[field]
		theirsVal := theirsBools[field]
		
		if conflict := m.detectConflict(field, baseVal, oursVal, theirsVal); conflict != nil {
			conflicts = append(conflicts, *conflict)
			// For boolean capabilities, true wins (OR logic)
			*mergedField = oursVal || theirsVal
			conflict.CanMerge = true
			conflict.Suggested = *mergedField
		} else if theirsVal != baseVal {
			*mergedField = theirsVal
		}
	}
	
	return merged, conflicts
}

// detectModalityConflict detects conflicts in modalities
func (m *threeWayMerger) detectModalityConflict(path string, base, ours, theirs catalogs.ModelModalities) *Conflict {
	if modalitiesEqual(ours, theirs) {
		return nil
	}
	
	oursChanged := !modalitiesEqual(base, ours)
	theirsChanged := !modalitiesEqual(base, theirs)
	
	if !oursChanged {
		return nil
	}
	if !theirsChanged {
		return nil
	}
	
	return &Conflict{
		Path:   path,
		Base:   base,
		Ours:   ours,
		Theirs: theirs,
		Type:   ConflictTypeModified,
	}
}

// modalitiesEqual compares two ModelModalities
func modalitiesEqual(a, b catalogs.ModelModalities) bool {
	if len(a.Input) != len(b.Input) || len(a.Output) != len(b.Output) {
		return false
	}
	
	// Compare input modalities
	inputMap := make(map[catalogs.ModelModality]bool)
	for _, m := range a.Input {
		inputMap[m] = true
	}
	for _, m := range b.Input {
		if !inputMap[m] {
			return false
		}
	}
	
	// Compare output modalities
	outputMap := make(map[catalogs.ModelModality]bool)
	for _, m := range a.Output {
		outputMap[m] = true
	}
	for _, m := range b.Output {
		if !outputMap[m] {
			return false
		}
	}
	
	return true
}

// unionModalities creates a union of two modalities
func unionModalities(a, b catalogs.ModelModalities) catalogs.ModelModalities {
	inputMap := make(map[catalogs.ModelModality]bool)
	outputMap := make(map[catalogs.ModelModality]bool)
	
	// Add all from a
	for _, m := range a.Input {
		inputMap[m] = true
	}
	for _, m := range a.Output {
		outputMap[m] = true
	}
	
	// Add all from b
	for _, m := range b.Input {
		inputMap[m] = true
	}
	for _, m := range b.Output {
		outputMap[m] = true
	}
	
	// Convert back to slices
	result := catalogs.ModelModalities{}
	for m := range inputMap {
		result.Input = append(result.Input, m)
	}
	for m := range outputMap {
		result.Output = append(result.Output, m)
	}
	
	return result
}

// mergeMetadata merges model metadata
func (m *threeWayMerger) mergeMetadata(base, ours, theirs *catalogs.ModelMetadata) (*catalogs.ModelMetadata, []Conflict) {
	conflicts := []Conflict{}
	
	// Handle nil cases
	if ours == nil && theirs == nil {
		return nil, conflicts
	}
	if ours == nil {
		return theirs, conflicts
	}
	if theirs == nil {
		return ours, conflicts
	}
	
	merged := &catalogs.ModelMetadata{}
	*merged = *ours // Start with our version
	
	if base == nil {
		base = &catalogs.ModelMetadata{}
	}
	
	// Release date - detect conflict only if both sides changed it differently
	if conflict := m.detectConflict("metadata.release_date", base.ReleaseDate, ours.ReleaseDate, theirs.ReleaseDate); conflict != nil {
		conflicts = append(conflicts, *conflict)
		// For release dates, prefer the earlier one (more accurate)
		if !ours.ReleaseDate.IsZero() && !theirs.ReleaseDate.IsZero() {
			if ours.ReleaseDate.Before(theirs.ReleaseDate) {
				merged.ReleaseDate = ours.ReleaseDate
			} else {
				merged.ReleaseDate = theirs.ReleaseDate
			}
			conflict.CanMerge = true
			conflict.Suggested = merged.ReleaseDate
		}
	} else if theirs.ReleaseDate != base.ReleaseDate {
		merged.ReleaseDate = theirs.ReleaseDate
	}
	
	// Open weights - boolean, true wins
	if conflict := m.detectConflict("metadata.open_weights", base.OpenWeights, ours.OpenWeights, theirs.OpenWeights); conflict != nil {
		conflicts = append(conflicts, *conflict)
		merged.OpenWeights = ours.OpenWeights || theirs.OpenWeights
		conflict.CanMerge = true
		conflict.Suggested = merged.OpenWeights
	} else if theirs.OpenWeights != base.OpenWeights {
		merged.OpenWeights = theirs.OpenWeights
	}
	
	return merged, conflicts
}

// trackMerge tracks provenance for three-way merge
func (m *threeWayMerger) trackMerge(merged, base, ours, theirs catalogs.Model) {
	if m.tracker == nil {
		return
	}
	
	// Track which source contributed to each field
	// This is simplified - real implementation would be more detailed
	m.tracker.Track(
		ResourceTypeModel,
		merged.ID,
		"three-way-merge",
		ProvenanceInfo{
			Source:    "three-way",
			Field:     "merge",
			Value:     merged,
			Timestamp: utcNow(),
		},
	)
}

// MergeProviders performs a three-way merge on providers
func (m *threeWayMerger) MergeProviders(base, ours, theirs catalogs.Provider) (catalogs.Provider, []Conflict, error) {
	merged := ours
	var conflicts []Conflict
	
	// Core identification
	if ours.ID != theirs.ID {
		conflicts = append(conflicts, Conflict{
			Path:   "id",
			Base:   string(base.ID),
			Ours:   string(ours.ID),
			Theirs: string(theirs.ID),
			Type:   ConflictTypeModified,
		})
	}
	
	// Name
	if conflict := m.detectConflict("name", base.Name, ours.Name, theirs.Name); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if theirs.Name != base.Name {
		merged.Name = theirs.Name
	}
	
	// Headquarters (Provider has Headquarters, not Description)
	if conflict := m.detectPtrConflict("headquarters", base.Headquarters, ours.Headquarters, theirs.Headquarters); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if !stringPtrEqual(base.Headquarters, theirs.Headquarters) {
		merged.Headquarters = theirs.Headquarters
	}
	
	// IconURL
	if conflict := m.detectPtrConflict("icon_url", base.IconURL, ours.IconURL, theirs.IconURL); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if !stringPtrEqual(base.IconURL, theirs.IconURL) {
		merged.IconURL = theirs.IconURL
	}
	
	// StatusPageURL
	if conflict := m.detectPtrConflict("status_page_url", base.StatusPageURL, ours.StatusPageURL, theirs.StatusPageURL); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if !stringPtrEqual(base.StatusPageURL, theirs.StatusPageURL) {
		merged.StatusPageURL = theirs.StatusPageURL
	}
	
	return merged, conflicts, nil
}

// MergeAuthors performs a three-way merge on authors
func (m *threeWayMerger) MergeAuthors(base, ours, theirs catalogs.Author) (catalogs.Author, []Conflict, error) {
	merged := ours
	var conflicts []Conflict
	
	// Core identification
	if ours.ID != theirs.ID {
		conflicts = append(conflicts, Conflict{
			Path:   "id",
			Base:   string(base.ID),
			Ours:   string(ours.ID),
			Theirs: string(theirs.ID),
			Type:   ConflictTypeModified,
		})
	}
	
	// Name
	if conflict := m.detectConflict("name", base.Name, ours.Name, theirs.Name); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if theirs.Name != base.Name {
		merged.Name = theirs.Name
	}
	
	// Description
	if conflict := m.detectPtrConflict("description", base.Description, ours.Description, theirs.Description); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if !stringPtrEqual(base.Description, theirs.Description) {
		merged.Description = theirs.Description
	}
	
	// Website (Author has Website, not URL)
	if conflict := m.detectPtrConflict("website", base.Website, ours.Website, theirs.Website); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if !stringPtrEqual(base.Website, theirs.Website) {
		merged.Website = theirs.Website
	}
	
	// HuggingFace
	if conflict := m.detectPtrConflict("huggingface", base.HuggingFace, ours.HuggingFace, theirs.HuggingFace); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if !stringPtrEqual(base.HuggingFace, theirs.HuggingFace) {
		merged.HuggingFace = theirs.HuggingFace
	}
	
	// GitHub
	if conflict := m.detectPtrConflict("github", base.GitHub, ours.GitHub, theirs.GitHub); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if !stringPtrEqual(base.GitHub, theirs.GitHub) {
		merged.GitHub = theirs.GitHub
	}
	
	// Twitter
	if conflict := m.detectPtrConflict("twitter", base.Twitter, ours.Twitter, theirs.Twitter); conflict != nil {
		conflicts = append(conflicts, *conflict)
	} else if !stringPtrEqual(base.Twitter, theirs.Twitter) {
		merged.Twitter = theirs.Twitter
	}
	
	return merged, conflicts, nil
}

// ResolveConflicts applies a conflict resolution strategy to a set of conflicts
func (m *threeWayMerger) ResolveConflicts(conflicts []Conflict, strategy ConflictResolution) []Resolution {
	resolutions := make([]Resolution, len(conflicts))
	
	for i, conflict := range conflicts {
		resolution := Resolution{
			Conflict: conflict,
			Decision: strategy,
		}
		
		switch strategy {
		case ResolutionOurs:
			resolution.Value = conflict.Ours
			resolution.Reason = "Taking our changes"
			
		case ResolutionTheirs:
			resolution.Value = conflict.Theirs
			resolution.Reason = "Taking their changes"
			
		case ResolutionBase:
			resolution.Value = conflict.Base
			resolution.Reason = "Keeping original value"
			
		case ResolutionMerge:
			if conflict.CanMerge {
				resolution.Value = conflict.Suggested
				resolution.Reason = "Automatic merge applied"
			} else {
				resolution.Value = conflict.Ours // Fallback to ours
				resolution.Reason = "Cannot auto-merge, using ours"
			}
			
		case ResolutionAuthority:
			// Use field authorities to determine winner
			if m.authorities != nil {
				authority := m.authorities.GetAuthority(conflict.Path, ResourceTypeModel)
				if authority != nil {
					resolution.Value = conflict.Theirs // Example based on authority
					resolution.Reason = fmt.Sprintf("Authority: %s", authority.Source)
				} else {
					resolution.Value = conflict.Ours
					resolution.Reason = "No authority defined, using ours"
				}
			} else {
				resolution.Value = conflict.Ours
				resolution.Reason = "No authorities configured, using ours"
			}
			
		default:
			resolution.Value = conflict.Ours
			resolution.Reason = "Unknown strategy, using ours"
		}
		
		resolutions[i] = resolution
	}
	
	return resolutions
}

// detectPtrConflict detects conflicts between string pointer values
func (m *threeWayMerger) detectPtrConflict(path string, base, ours, theirs *string) *Conflict {
	baseVal := stringPtrValue(base)
	oursVal := stringPtrValue(ours)
	theirsVal := stringPtrValue(theirs)
	
	return m.detectConflict(path, baseVal, oursVal, theirsVal)
}

// stringPtrEqual compares two string pointers
func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// stringPtrValue safely gets the value of a string pointer
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}