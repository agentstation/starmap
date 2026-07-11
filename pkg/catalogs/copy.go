package catalogs

func copyPtr[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func copyMap[K comparable, V any](input map[K]V) map[K]V {
	if input == nil {
		return nil
	}
	result := make(map[K]V, len(input))
	for k, v := range input {
		result[k] = v
	}
	return result
}

// DeepCopyProviderModels creates a deep copy of a provider's Models map.
// Returns nil if the input map is nil.
func DeepCopyProviderModels(models map[string]*Model) map[string]*Model {
	if models == nil {
		return nil
	}

	result := make(map[string]*Model, len(models))
	for k, v := range models {
		if v != nil {
			modelCopy := DeepCopyModel(*v)
			result[k] = &modelCopy
		} else {
			result[k] = nil
		}
	}
	return result
}

// DeepCopyAuthorModels creates a deep copy of an author's Models map.
// Returns nil if the input map is nil.
func DeepCopyAuthorModels(models map[string]*Model) map[string]*Model {
	if models == nil {
		return nil
	}

	result := make(map[string]*Model, len(models))
	for k, v := range models {
		if v != nil {
			modelCopy := DeepCopyModel(*v)
			result[k] = &modelCopy
		} else {
			result[k] = nil
		}
	}
	return result
}

// DeepCopyModel creates a deep copy of a Model.
func DeepCopyModel(model Model) Model {
	modelCopy := model
	modelCopy.Authors = deepCopyModelAuthors(model.Authors)
	modelCopy.Metadata = deepCopyModelMetadata(model.Metadata)
	modelCopy.Lineage = deepCopyModelLineage(model.Lineage)
	modelCopy.Features = deepCopyModelFeatures(model.Features)
	modelCopy.Attachments = deepCopyModelAttachments(model.Attachments)
	modelCopy.Generation = deepCopyModelGeneration(model.Generation)
	modelCopy.Reasoning = deepCopyModelControlLevels(model.Reasoning)
	modelCopy.ReasoningTokens = copyPtr(model.ReasoningTokens)
	modelCopy.Verbosity = deepCopyModelControlLevels(model.Verbosity)
	modelCopy.Tools = deepCopyModelTools(model.Tools)
	modelCopy.Delivery = deepCopyModelDelivery(model.Delivery)
	modelCopy.Modes = deepCopyModelModes(model.Modes)
	modelCopy.Pricing = deepCopyModelPricing(model.Pricing)
	modelCopy.Limits = copyPtr(model.Limits)
	modelCopy.Extensions = model.Extensions.Copy()
	return modelCopy
}

// DeepCopyAuthors creates a deep copy of an Author slice.
func DeepCopyAuthors(authors []Author) []Author {
	if authors == nil {
		return nil
	}
	result := make([]Author, len(authors))
	for i, author := range authors {
		result[i] = DeepCopyAuthor(author)
	}
	return result
}

func deepCopyModelAuthors(authors []Author) []Author {
	if authors == nil {
		return nil
	}
	result := make([]Author, len(authors))
	for i, author := range authors {
		result[i] = deepCopyAuthorMetadata(author)
	}
	return result
}

// DeepCopyProvider creates a deep copy of a Provider including its Models map.
func DeepCopyProvider(provider Provider) Provider {
	providerCopy := provider
	providerCopy.Aliases = append([]ProviderID(nil), provider.Aliases...)
	providerCopy.Headquarters = copyPtr(provider.Headquarters)
	providerCopy.IconURL = copyPtr(provider.IconURL)
	providerCopy.APIKey = copyPtr(provider.APIKey)
	providerCopy.EnvVars = append([]ProviderEnvVar(nil), provider.EnvVars...)
	providerCopy.Catalog = deepCopyProviderCatalog(provider.Catalog)
	providerCopy.Models = DeepCopyProviderModels(provider.Models)
	providerCopy.StatusPageURL = copyPtr(provider.StatusPageURL)
	providerCopy.ChatCompletions = deepCopyProviderChatCompletions(provider.ChatCompletions)
	providerCopy.PrivacyPolicy = deepCopyProviderPrivacyPolicy(provider.PrivacyPolicy)
	providerCopy.RetentionPolicy = deepCopyProviderRetentionPolicy(provider.RetentionPolicy)
	providerCopy.GovernancePolicy = deepCopyProviderGovernancePolicy(provider.GovernancePolicy)
	providerCopy.Extensions = provider.Extensions.Copy()
	providerCopy.EnvVarValues = copyMap(provider.EnvVarValues)
	return providerCopy
}

// DeepCopyAuthor creates a deep copy of an Author including its Models map.
func DeepCopyAuthor(author Author) Author {
	authorCopy := deepCopyAuthorMetadata(author)
	authorCopy.Models = DeepCopyAuthorModels(author.Models)
	return authorCopy
}

// DeepCopyEndpoint creates a copy of an Endpoint.
func DeepCopyEndpoint(endpoint Endpoint) Endpoint {
	return endpoint
}

func deepCopyAuthorMetadata(author Author) Author {
	authorCopy := author
	authorCopy.Aliases = append([]AuthorID(nil), author.Aliases...)
	authorCopy.Description = copyPtr(author.Description)
	authorCopy.Headquarters = copyPtr(author.Headquarters)
	authorCopy.IconURL = copyPtr(author.IconURL)
	authorCopy.Website = copyPtr(author.Website)
	authorCopy.HuggingFace = copyPtr(author.HuggingFace)
	authorCopy.GitHub = copyPtr(author.GitHub)
	authorCopy.Twitter = copyPtr(author.Twitter)
	authorCopy.Catalog = deepCopyAuthorCatalog(author.Catalog)
	authorCopy.Models = nil
	return authorCopy
}

func deepCopyModelMetadata(metadata *ModelMetadata) *ModelMetadata {
	if metadata == nil {
		return nil
	}
	copied := *metadata
	copied.KnowledgeCutoff = copyPtr(metadata.KnowledgeCutoff)
	copied.Tags = append([]ModelTag(nil), metadata.Tags...)
	copied.Architecture = deepCopyModelArchitecture(metadata.Architecture)
	return &copied
}

func deepCopyModelArchitecture(architecture *ModelArchitecture) *ModelArchitecture {
	if architecture == nil {
		return nil
	}
	copied := *architecture
	copied.Precision = copyPtr(architecture.Precision)
	copied.BaseModel = copyPtr(architecture.BaseModel)
	return &copied
}

func deepCopyModelLineage(lineage *ModelLineage) *ModelLineage {
	if lineage == nil {
		return nil
	}
	copied := *lineage
	copied.Root = copyPtr(lineage.Root)
	copied.Parent = copyPtr(lineage.Parent)
	return &copied
}

func deepCopyModelFeatures(features *ModelFeatures) *ModelFeatures {
	if features == nil {
		return nil
	}
	copied := *features
	copied.Modalities.Input = append([]ModelModality(nil), features.Modalities.Input...)
	copied.Modalities.Output = append([]ModelModality(nil), features.Modalities.Output...)
	return &copied
}

func deepCopyModelAttachments(attachments *ModelAttachments) *ModelAttachments {
	if attachments == nil {
		return nil
	}
	copied := *attachments
	copied.MimeTypes = append([]string(nil), attachments.MimeTypes...)
	copied.MaxFileSize = copyPtr(attachments.MaxFileSize)
	copied.MaxFiles = copyPtr(attachments.MaxFiles)
	return &copied
}

func deepCopyModelGeneration(generation *ModelGeneration) *ModelGeneration {
	if generation == nil {
		return nil
	}
	copied := *generation
	copied.Temperature = copyPtr(generation.Temperature)
	copied.TopP = copyPtr(generation.TopP)
	copied.TopK = copyPtr(generation.TopK)
	copied.TopA = copyPtr(generation.TopA)
	copied.MinP = copyPtr(generation.MinP)
	copied.TypicalP = copyPtr(generation.TypicalP)
	copied.TFS = copyPtr(generation.TFS)
	copied.MaxTokens = copyPtr(generation.MaxTokens)
	copied.MaxOutputTokens = copyPtr(generation.MaxOutputTokens)
	copied.FrequencyPenalty = copyPtr(generation.FrequencyPenalty)
	copied.PresencePenalty = copyPtr(generation.PresencePenalty)
	copied.RepetitionPenalty = copyPtr(generation.RepetitionPenalty)
	copied.NoRepeatNgramSize = copyPtr(generation.NoRepeatNgramSize)
	copied.LengthPenalty = copyPtr(generation.LengthPenalty)
	copied.TopLogprobs = copyPtr(generation.TopLogprobs)
	copied.N = copyPtr(generation.N)
	copied.BestOf = copyPtr(generation.BestOf)
	copied.MirostatTau = copyPtr(generation.MirostatTau)
	copied.MirostatEta = copyPtr(generation.MirostatEta)
	copied.ContrastiveSearchPenaltyAlpha = copyPtr(generation.ContrastiveSearchPenaltyAlpha)
	copied.NumBeams = copyPtr(generation.NumBeams)
	copied.DiversityPenalty = copyPtr(generation.DiversityPenalty)
	return &copied
}

func deepCopyModelControlLevels(levels *ModelControlLevels) *ModelControlLevels {
	if levels == nil {
		return nil
	}
	copied := *levels
	copied.Levels = append([]ModelControlLevel(nil), levels.Levels...)
	copied.Default = copyPtr(levels.Default)
	return &copied
}

func deepCopyModelTools(tools *ModelTools) *ModelTools {
	if tools == nil {
		return nil
	}
	copied := *tools
	copied.ToolChoices = append([]ToolChoice(nil), tools.ToolChoices...)
	copied.WebSearch = deepCopyModelWebSearch(tools.WebSearch)
	return &copied
}

func deepCopyModelWebSearch(search *ModelWebSearch) *ModelWebSearch {
	if search == nil {
		return nil
	}
	copied := *search
	copied.MaxResults = copyPtr(search.MaxResults)
	copied.SearchPrompt = copyPtr(search.SearchPrompt)
	copied.SearchContextSizes = append([]ModelControlLevel(nil), search.SearchContextSizes...)
	copied.DefaultContextSize = copyPtr(search.DefaultContextSize)
	return &copied
}

func deepCopyModelDelivery(delivery *ModelDelivery) *ModelDelivery {
	if delivery == nil {
		return nil
	}
	copied := *delivery
	copied.Protocols = append([]ModelResponseProtocol(nil), delivery.Protocols...)
	copied.Streaming = append([]ModelStreaming(nil), delivery.Streaming...)
	copied.Formats = append([]ModelResponseFormat(nil), delivery.Formats...)
	return &copied
}

func deepCopyModelModes(modes map[string]ModelMode) map[string]ModelMode {
	if modes == nil {
		return nil
	}
	copied := make(map[string]ModelMode, len(modes))
	for name, mode := range modes {
		copied[name] = ModelMode{
			Pricing:  deepCopyModelPricing(mode.Pricing),
			Provider: deepCopyModelProviderMode(mode.Provider),
		}
	}
	return copied
}

func deepCopyModelProviderMode(mode *ModelProviderMode) *ModelProviderMode {
	if mode == nil {
		return nil
	}
	copied := *mode
	copied.Headers = copyMap(mode.Headers)
	copied.Body = deepCopyExtensionMap(mode.Body)
	return &copied
}

func deepCopyModelPricing(pricing *ModelPricing) *ModelPricing {
	if pricing == nil {
		return nil
	}
	copied := *pricing
	copied.EffectiveFrom = copyPtr(pricing.EffectiveFrom)
	copied.EffectiveUntil = copyPtr(pricing.EffectiveUntil)
	copied.Tokens = deepCopyModelTokenPricing(pricing.Tokens)
	copied.Operations = deepCopyModelOperationPricing(pricing.Operations)
	copied.Tiers = deepCopyModelPricingTiers(pricing.Tiers)
	return &copied
}

func deepCopyModelPricingTiers(tiers []ModelPricingTier) []ModelPricingTier {
	if tiers == nil {
		return nil
	}
	copied := make([]ModelPricingTier, len(tiers))
	for i, tier := range tiers {
		copied[i] = tier
		copied[i].Tokens = deepCopyModelTokenPricing(tier.Tokens)
		copied[i].Operations = deepCopyModelOperationPricing(tier.Operations)
	}
	return copied
}

func deepCopyModelTokenPricing(pricing *ModelTokenPricing) *ModelTokenPricing {
	if pricing == nil {
		return nil
	}
	copied := *pricing
	copied.Input = copyPtr(pricing.Input)
	copied.Output = copyPtr(pricing.Output)
	copied.Reasoning = copyPtr(pricing.Reasoning)
	copied.Cache = deepCopyModelTokenCachePricing(pricing.Cache)
	copied.CacheRead = copyPtr(pricing.CacheRead)
	copied.CacheWrite = copyPtr(pricing.CacheWrite)
	return &copied
}

func deepCopyModelTokenCachePricing(pricing *ModelTokenCachePricing) *ModelTokenCachePricing {
	if pricing == nil {
		return nil
	}
	copied := *pricing
	copied.Read = copyPtr(pricing.Read)
	copied.Write = copyPtr(pricing.Write)
	return &copied
}

func deepCopyModelOperationPricing(pricing *ModelOperationPricing) *ModelOperationPricing {
	if pricing == nil {
		return nil
	}
	copied := *pricing
	copied.Request = copyPtr(pricing.Request)
	copied.ImageInput = copyPtr(pricing.ImageInput)
	copied.AudioInput = copyPtr(pricing.AudioInput)
	copied.VideoInput = copyPtr(pricing.VideoInput)
	copied.ImageGen = copyPtr(pricing.ImageGen)
	copied.AudioGen = copyPtr(pricing.AudioGen)
	copied.VideoGen = copyPtr(pricing.VideoGen)
	copied.WebSearch = copyPtr(pricing.WebSearch)
	copied.FunctionCall = copyPtr(pricing.FunctionCall)
	copied.ToolUse = copyPtr(pricing.ToolUse)
	return &copied
}

func deepCopyProviderCatalog(catalog *ProviderCatalog) *ProviderCatalog {
	if catalog == nil {
		return nil
	}
	copied := *catalog
	copied.Docs = copyPtr(catalog.Docs)
	copied.Endpoint.FieldMappings = append([]FieldMapping(nil), catalog.Endpoint.FieldMappings...)
	copied.Endpoint.FeatureRules = deepCopyFeatureRules(catalog.Endpoint.FeatureRules)
	copied.Endpoint.AuthorMapping = deepCopyAuthorMapping(catalog.Endpoint.AuthorMapping)
	copied.Authors = append([]AuthorID(nil), catalog.Authors...)
	return &copied
}

func deepCopyFeatureRules(rules []FeatureRule) []FeatureRule {
	if rules == nil {
		return nil
	}
	copied := make([]FeatureRule, len(rules))
	for i, rule := range rules {
		copied[i] = rule
		copied[i].Contains = append([]string(nil), rule.Contains...)
	}
	return copied
}

func deepCopyAuthorMapping(mapping *AuthorMapping) *AuthorMapping {
	if mapping == nil {
		return nil
	}
	copied := *mapping
	copied.Normalized = copyMap(mapping.Normalized)
	return &copied
}

func deepCopyProviderChatCompletions(chat *ProviderChatCompletions) *ProviderChatCompletions {
	if chat == nil {
		return nil
	}
	copied := *chat
	copied.URL = copyPtr(chat.URL)
	copied.HealthAPIURL = copyPtr(chat.HealthAPIURL)
	copied.HealthComponents = append([]ProviderHealthComponent(nil), chat.HealthComponents...)
	return &copied
}

func deepCopyProviderPrivacyPolicy(policy *ProviderPrivacyPolicy) *ProviderPrivacyPolicy {
	if policy == nil {
		return nil
	}
	copied := *policy
	copied.PrivacyPolicyURL = copyPtr(policy.PrivacyPolicyURL)
	copied.TermsOfServiceURL = copyPtr(policy.TermsOfServiceURL)
	copied.RetainsData = copyPtr(policy.RetainsData)
	copied.TrainsOnData = copyPtr(policy.TrainsOnData)
	return &copied
}

func deepCopyProviderRetentionPolicy(policy *ProviderRetentionPolicy) *ProviderRetentionPolicy {
	if policy == nil {
		return nil
	}
	copied := *policy
	copied.Duration = copyPtr(policy.Duration)
	copied.Details = copyPtr(policy.Details)
	return &copied
}

func deepCopyProviderGovernancePolicy(policy *ProviderGovernancePolicy) *ProviderGovernancePolicy {
	if policy == nil {
		return nil
	}
	copied := *policy
	copied.ModerationRequired = copyPtr(policy.ModerationRequired)
	copied.Moderated = copyPtr(policy.Moderated)
	copied.Moderator = copyPtr(policy.Moderator)
	return &copied
}

func deepCopyAuthorCatalog(catalog *AuthorCatalog) *AuthorCatalog {
	if catalog == nil {
		return nil
	}
	copied := *catalog
	copied.Description = copyPtr(catalog.Description)
	copied.Attribution = deepCopyAuthorAttribution(catalog.Attribution)
	return &copied
}

func deepCopyAuthorAttribution(attribution *AuthorAttribution) *AuthorAttribution {
	if attribution == nil {
		return nil
	}
	copied := *attribution
	copied.Patterns = append([]string(nil), attribution.Patterns...)
	return &copied
}

// ShallowCopyProviderModels creates a shallow copy of a provider's Models map.
// The map is copied but Model pointers are shared.
// Returns nil if the input map is nil.
func ShallowCopyProviderModels(models map[string]*Model) map[string]*Model {
	if models == nil {
		return nil
	}

	result := make(map[string]*Model, len(models))
	for k, v := range models {
		result[k] = v
	}
	return result
}

// ShallowCopyAuthorModels creates a shallow copy of an author's Models map.
// The map is copied but Model pointers are shared.
// Returns nil if the input map is nil.
func ShallowCopyAuthorModels(models map[string]*Model) map[string]*Model {
	if models == nil {
		return nil
	}

	result := make(map[string]*Model, len(models))
	for k, v := range models {
		result[k] = v
	}
	return result
}
