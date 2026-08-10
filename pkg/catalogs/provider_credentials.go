package catalogs

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ProviderCredentialFieldID identifies one secret or non-secret credential
// field. Values are runtime state and are not part of the catalog.
type ProviderCredentialFieldID string

// ProviderCredentialProfileID identifies one authentication profile.
type ProviderCredentialProfileID string

// ProviderCredentials defines credential fields once and composes them into
// named profiles. Each plane lists its permitted profiles in selection order.
type ProviderCredentials struct {
	Fields             []ProviderCredentialField   `json:"fields" yaml:"fields"`
	Profiles           []ProviderCredentialProfile `json:"profiles" yaml:"profiles"`
	CatalogAcquisition ProviderCredentialPlane     `json:"catalog_acquisition" yaml:"catalog_acquisition"`
	Inference          ProviderCredentialPlane     `json:"inference" yaml:"inference"`
}

// ProviderCredentialPlane defines the ordered authentication profiles that one
// credential plane permits. Selecting a profile is terminal; the catalog does
// not define automatic fallback between profiles.
type ProviderCredentialPlane struct {
	Required     bool                          `json:"required" yaml:"required"`
	Alternatives []ProviderCredentialProfileID `json:"alternatives" yaml:"alternatives"`
}

// ProviderCredentialFieldKind distinguishes secret material from non-secret
// endpoint and protocol parameters.
type ProviderCredentialFieldKind string

const (
	// ProviderCredentialFieldSecret is sensitive authentication material.
	ProviderCredentialFieldSecret ProviderCredentialFieldKind = "secret"
	// ProviderCredentialFieldParameter is non-secret runtime configuration.
	ProviderCredentialFieldParameter ProviderCredentialFieldKind = "parameter"
)

// ProviderCredentialField defines one named material field and its conventional
// ambient environment names. Product-specific names are derived from its ID.
type ProviderCredentialField struct {
	ID          ProviderCredentialFieldID   `json:"id" yaml:"id"`
	Kind        ProviderCredentialFieldKind `json:"kind" yaml:"kind"`
	Required    bool                        `json:"required" yaml:"required"`
	Environment []string                    `json:"environment,omitempty" yaml:"environment,omitempty"`
	Pattern     string                      `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Description string                      `json:"description,omitempty" yaml:"description,omitempty"`
}

// ProviderAuthenticationPrimitive identifies compiled authentication behavior.
// It never identifies a provider.
type ProviderAuthenticationPrimitive string

const (
	// ProviderAuthenticationNone sends no authentication material.
	ProviderAuthenticationNone ProviderAuthenticationPrimitive = "none"
	// ProviderAuthenticationAPIKey places a static API key on a request.
	ProviderAuthenticationAPIKey ProviderAuthenticationPrimitive = "api-key"
	// ProviderAuthenticationBearerToken places a resolved bearer token.
	ProviderAuthenticationBearerToken ProviderAuthenticationPrimitive = "bearer-token"
	// ProviderAuthenticationGoogleDefault uses Google's default credential chain.
	ProviderAuthenticationGoogleDefault ProviderAuthenticationPrimitive = "google-default"
	// ProviderAuthenticationAzureDefault uses Azure's default credential chain.
	ProviderAuthenticationAzureDefault ProviderAuthenticationPrimitive = "azure-default"
	// ProviderAuthenticationAWSDefault uses AWS's default credential chain.
	ProviderAuthenticationAWSDefault ProviderAuthenticationPrimitive = "aws-default"
)

// ProviderCredentialProfile defines one complete authentication alternative.
// Field references share provider-level definitions across alternatives.
type ProviderCredentialProfile struct {
	ID               ProviderCredentialProfileID           `json:"id" yaml:"id"`
	Primitive        ProviderAuthenticationPrimitive       `json:"primitive" yaml:"primitive"`
	Fields           []ProviderCredentialFieldID           `json:"fields,omitempty" yaml:"fields,omitempty"`
	Placements       []ProviderCredentialPlacement         `json:"placements,omitempty" yaml:"placements,omitempty"`
	Scopes           []string                              `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	EndpointBindings []ProviderCredentialEndpointBinding   `json:"endpoint_bindings,omitempty" yaml:"endpoint_bindings,omitempty"`
	ProtocolOptions  ProviderAuthenticationProtocolOptions `json:"protocol_options,omitempty" yaml:"protocol_options,omitempty"`
}

// ProviderCredentialPlacementKind identifies where one credential field is
// applied to an HTTP request.
type ProviderCredentialPlacementKind string

const (
	// ProviderCredentialPlacementHeader applies material to an HTTP header.
	ProviderCredentialPlacementHeader ProviderCredentialPlacementKind = "header"
	// ProviderCredentialPlacementQuery applies material to a URL query value.
	ProviderCredentialPlacementQuery ProviderCredentialPlacementKind = "query"
)

// ProviderCredentialScheme identifies the transformation applied before a
// credential field is placed on a request.
type ProviderCredentialScheme string

const (
	// ProviderCredentialSchemeDirect places bytes without a prefix.
	ProviderCredentialSchemeDirect ProviderCredentialScheme = "direct"
	// ProviderCredentialSchemeBearer adds the Bearer authentication prefix.
	ProviderCredentialSchemeBearer ProviderCredentialScheme = "bearer"
	// ProviderCredentialSchemeBasic adds the Basic authentication prefix.
	ProviderCredentialSchemeBasic ProviderCredentialScheme = "basic"
)

// ProviderCredentialPlacement binds one resolved field to a request location.
// Query placement requires an HTTPS provider-evidence URL.
type ProviderCredentialPlacement struct {
	Field       ProviderCredentialFieldID       `json:"field" yaml:"field"`
	Kind        ProviderCredentialPlacementKind `json:"kind" yaml:"kind"`
	Name        string                          `json:"name" yaml:"name"`
	Scheme      ProviderCredentialScheme        `json:"scheme" yaml:"scheme"`
	EvidenceURL string                          `json:"evidence_url,omitempty" yaml:"evidence_url,omitempty"`
}

// ProviderCredentialEndpointBinding binds one non-secret field to a named URL
// template variable.
type ProviderCredentialEndpointBinding struct {
	Field    ProviderCredentialFieldID `json:"field" yaml:"field"`
	Variable string                    `json:"variable" yaml:"variable"`
}

// ProviderAuthenticationProtocolOptions is a typed union of primitive-owned
// protocol settings. Provider membership does not belong in this union.
type ProviderAuthenticationProtocolOptions struct {
	GoogleDefault *ProviderGoogleDefaultProtocolOptions `json:"google_default,omitempty" yaml:"google_default,omitempty"`
	AWSDefault    *ProviderAWSDefaultProtocolOptions    `json:"aws_default,omitempty" yaml:"aws_default,omitempty"`
}

// ProviderGoogleDefaultProtocolOptions configures Google token application.
type ProviderGoogleDefaultProtocolOptions struct {
	QuotaProjectField ProviderCredentialFieldID `json:"quota_project_field,omitempty" yaml:"quota_project_field,omitempty"`
}

// ProviderAWSDefaultProtocolOptions configures AWS request signing.
type ProviderAWSDefaultProtocolOptions struct {
	RegionField ProviderCredentialFieldID `json:"region_field" yaml:"region_field"`
	Service     string                    `json:"service" yaml:"service"`
}

var (
	credentialIdentifierPattern  = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	credentialEnvironmentPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	credentialProductPattern     = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
	httpHeaderNamePattern        = regexp.MustCompile("^[!#$%&'*+\\-.^_`|~0-9A-Za-z]+$")
	queryNamePattern             = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.~-]*$`)
	endpointVariablePattern      = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	awsServicePattern            = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

var protectedCredentialHeaders = map[string]struct{}{
	"connection":          {},
	"content-length":      {},
	"cookie":              {},
	"forwarded":           {},
	"host":                {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"set-cookie":          {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

// DerivedCredentialEnvironmentName derives a product-specific ambient name.
// It validates all components before it replaces ID separators with underscores.
func DerivedCredentialEnvironmentName(
	product string,
	providerID ProviderID,
	fieldID ProviderCredentialFieldID,
) (string, error) {
	if !credentialProductPattern.MatchString(product) {
		return "", providerContractError("product", product, "must contain uppercase letters and digits")
	}
	if !validCredentialIdentifier(string(providerID)) {
		return "", providerContractError("provider_id", providerID, "must be a lowercase kebab-case ID")
	}
	if !validCredentialIdentifier(string(fieldID)) {
		return "", providerContractError("field_id", fieldID, "must be a lowercase kebab-case ID")
	}
	return product + "_" + credentialEnvironmentSuffix(providerID, fieldID), nil
}

func credentialEnvironmentSuffix(providerID ProviderID, fieldID ProviderCredentialFieldID) string {
	id := string(providerID) + "_" + string(fieldID)
	return strings.ToUpper(strings.ReplaceAll(id, "-", "_"))
}

func validCredentialIdentifier(value string) bool {
	return credentialIdentifierPattern.MatchString(value)
}

func (credentials *ProviderCredentials) validate() error {
	if credentials == nil {
		return nil
	}
	if len(credentials.Profiles) == 0 {
		return credentialError("profiles", nil, "must contain at least one profile")
	}

	fields := make(map[ProviderCredentialFieldID]ProviderCredentialField, len(credentials.Fields))
	environments := make(map[string]ProviderCredentialFieldID)
	for index, field := range credentials.Fields {
		path := fmt.Sprintf("fields[%d]", index)
		if !validCredentialIdentifier(string(field.ID)) {
			return credentialError(path+".id", field.ID, "must be a lowercase kebab-case ID")
		}
		if _, exists := fields[field.ID]; exists {
			return credentialError(path+".id", field.ID, "must be unique")
		}
		if field.Kind != ProviderCredentialFieldSecret && field.Kind != ProviderCredentialFieldParameter {
			return credentialError(path+".kind", field.Kind, "must be secret or parameter")
		}
		if field.Pattern != "" {
			if _, err := regexp.Compile(field.Pattern); err != nil {
				return credentialError(path+".pattern", field.Pattern, "must be a valid regular expression")
			}
		}
		for environmentIndex, environment := range field.Environment {
			environmentPath := fmt.Sprintf("%s.environment[%d]", path, environmentIndex)
			if !credentialEnvironmentPattern.MatchString(environment) {
				return credentialError(environmentPath, environment, "must be an uppercase environment name")
			}
			if owner, exists := environments[environment]; exists {
				return credentialError(environmentPath, environment, fmt.Sprintf("is already owned by field %q", owner))
			}
			environments[environment] = field.ID
		}
		fields[field.ID] = field
	}

	profiles := make(map[ProviderCredentialProfileID]ProviderCredentialProfile, len(credentials.Profiles))
	for index, profile := range credentials.Profiles {
		path := fmt.Sprintf("profiles[%d]", index)
		if !validCredentialIdentifier(string(profile.ID)) {
			return credentialError(path+".id", profile.ID, "must be a lowercase kebab-case ID")
		}
		if _, exists := profiles[profile.ID]; exists {
			return credentialError(path+".id", profile.ID, "must be unique")
		}
		if !validAuthenticationPrimitive(profile.Primitive) {
			return credentialError(path+".primitive", profile.Primitive, "is not supported")
		}
		if err := validateCredentialProfile(path, profile, fields); err != nil {
			return err
		}
		profiles[profile.ID] = profile
	}

	referenced := make(map[ProviderCredentialProfileID]struct{}, len(profiles))
	if err := validateCredentialPlane("catalog_acquisition", credentials.CatalogAcquisition, profiles, referenced); err != nil {
		return err
	}
	if err := validateCredentialPlane("inference", credentials.Inference, profiles, referenced); err != nil {
		return err
	}
	for profileID := range profiles {
		if _, exists := referenced[profileID]; !exists {
			return credentialError("profiles", profileID, "profile is not referenced by a credential plane")
		}
	}

	return nil
}

func validateCredentialProfile(
	path string,
	profile ProviderCredentialProfile,
	fields map[ProviderCredentialFieldID]ProviderCredentialField,
) error {
	profileFields := make(map[ProviderCredentialFieldID]struct{}, len(profile.Fields))
	for index, fieldID := range profile.Fields {
		fieldPath := fmt.Sprintf("%s.fields[%d]", path, index)
		if _, exists := fields[fieldID]; !exists {
			return credentialError(fieldPath, fieldID, "does not reference a declared field")
		}
		if _, exists := profileFields[fieldID]; exists {
			return credentialError(fieldPath, fieldID, "must be unique within the profile")
		}
		profileFields[fieldID] = struct{}{}
	}

	placements := make(map[string]struct{}, len(profile.Placements))
	for index, placement := range profile.Placements {
		placementPath := fmt.Sprintf("%s.placements[%d]", path, index)
		if err := validateProfileFieldReference(placementPath+".field", placement.Field, fields, profileFields); err != nil {
			return err
		}
		if err := validateCredentialPlacement(placementPath, placement); err != nil {
			return err
		}
		key := string(placement.Kind) + ":" + strings.ToLower(placement.Name)
		if _, exists := placements[key]; exists {
			return credentialError(placementPath+".name", placement.Name, "placement target is ambiguous")
		}
		placements[key] = struct{}{}
	}

	scopes := make(map[string]struct{}, len(profile.Scopes))
	for index, scope := range profile.Scopes {
		scopePath := fmt.Sprintf("%s.scopes[%d]", path, index)
		if strings.TrimSpace(scope) == "" {
			return credentialError(scopePath, scope, "must not be empty")
		}
		if _, exists := scopes[scope]; exists {
			return credentialError(scopePath, scope, "must be unique")
		}
		scopes[scope] = struct{}{}
	}

	bindings := make(map[string]struct{}, len(profile.EndpointBindings))
	for index, binding := range profile.EndpointBindings {
		bindingPath := fmt.Sprintf("%s.endpoint_bindings[%d]", path, index)
		if err := validateProfileFieldReference(bindingPath+".field", binding.Field, fields, profileFields); err != nil {
			return err
		}
		if fields[binding.Field].Kind != ProviderCredentialFieldParameter {
			return credentialError(bindingPath+".field", binding.Field, "must reference a parameter field")
		}
		if !endpointVariablePattern.MatchString(binding.Variable) {
			return credentialError(bindingPath+".variable", binding.Variable, "must be a lowercase template variable")
		}
		if _, exists := bindings[binding.Variable]; exists {
			return credentialError(bindingPath+".variable", binding.Variable, "endpoint binding is ambiguous")
		}
		bindings[binding.Variable] = struct{}{}
	}

	if err := validateAuthenticationProtocolOptions(path, profile, fields, profileFields); err != nil {
		return err
	}
	return validateAuthenticationPrimitiveContract(path, profile, fields)
}

func validateProfileFieldReference(
	path string,
	fieldID ProviderCredentialFieldID,
	fields map[ProviderCredentialFieldID]ProviderCredentialField,
	profileFields map[ProviderCredentialFieldID]struct{},
) error {
	if _, exists := fields[fieldID]; !exists {
		return credentialError(path, fieldID, "does not reference a declared field")
	}
	if _, exists := profileFields[fieldID]; !exists {
		return credentialError(path, fieldID, "does not belong to the profile")
	}
	return nil
}

func validateCredentialPlacement(path string, placement ProviderCredentialPlacement) error {
	if placement.Scheme != ProviderCredentialSchemeDirect &&
		placement.Scheme != ProviderCredentialSchemeBearer &&
		placement.Scheme != ProviderCredentialSchemeBasic {
		return credentialError(path+".scheme", placement.Scheme, "must be direct, bearer, or basic")
	}

	switch placement.Kind {
	case ProviderCredentialPlacementHeader:
		if !httpHeaderNamePattern.MatchString(placement.Name) {
			return credentialError(path+".name", placement.Name, "must be a valid HTTP header name")
		}
		name := strings.ToLower(placement.Name)
		if _, protected := protectedCredentialHeaders[name]; protected ||
			strings.HasPrefix(name, "proxy-") ||
			strings.HasPrefix(name, "sec-") ||
			strings.HasPrefix(name, "x-forwarded-") {
			return credentialError(path+".name", placement.Name, "is a protected HTTP header")
		}
		if placement.EvidenceURL != "" {
			return credentialError(path+".evidence_url", placement.EvidenceURL, "is only valid for query placement")
		}
	case ProviderCredentialPlacementQuery:
		if !queryNamePattern.MatchString(placement.Name) {
			return credentialError(path+".name", placement.Name, "must be a safe query parameter name")
		}
		if placement.Scheme != ProviderCredentialSchemeDirect {
			return credentialError(path+".scheme", placement.Scheme, "query placement requires the direct scheme")
		}
		evidence, err := url.Parse(placement.EvidenceURL)
		if err != nil || evidence.Scheme != "https" || evidence.Host == "" || evidence.User != nil {
			return credentialError(path+".evidence_url", placement.EvidenceURL, "must be an HTTPS provider-evidence URL")
		}
	default:
		return credentialError(path+".kind", placement.Kind, "must be header or query")
	}
	return nil
}

func validateAuthenticationProtocolOptions(
	path string,
	profile ProviderCredentialProfile,
	fields map[ProviderCredentialFieldID]ProviderCredentialField,
	profileFields map[ProviderCredentialFieldID]struct{},
) error {
	optionCount := 0
	if profile.ProtocolOptions.GoogleDefault != nil {
		optionCount++
		if profile.Primitive != ProviderAuthenticationGoogleDefault {
			return credentialError(path+".protocol_options.google_default", nil, "requires the google-default primitive")
		}
		fieldID := profile.ProtocolOptions.GoogleDefault.QuotaProjectField
		if fieldID != "" {
			optionPath := path + ".protocol_options.google_default.quota_project_field"
			if err := validateProfileFieldReference(optionPath, fieldID, fields, profileFields); err != nil {
				return err
			}
			if fields[fieldID].Kind != ProviderCredentialFieldParameter {
				return credentialError(optionPath, fieldID, "must reference a parameter field")
			}
		}
	}
	if profile.ProtocolOptions.AWSDefault != nil {
		optionCount++
		if profile.Primitive != ProviderAuthenticationAWSDefault {
			return credentialError(path+".protocol_options.aws_default", nil, "requires the aws-default primitive")
		}
		options := profile.ProtocolOptions.AWSDefault
		optionPath := path + ".protocol_options.aws_default.region_field"
		if err := validateProfileFieldReference(optionPath, options.RegionField, fields, profileFields); err != nil {
			return err
		}
		if fields[options.RegionField].Kind != ProviderCredentialFieldParameter {
			return credentialError(optionPath, options.RegionField, "must reference a parameter field")
		}
		if !awsServicePattern.MatchString(options.Service) {
			return credentialError(path+".protocol_options.aws_default.service", options.Service, "must be a lowercase AWS service name")
		}
	}
	if optionCount > 1 {
		return credentialError(path+".protocol_options", nil, "must select at most one protocol option")
	}
	return nil
}

func validateAuthenticationPrimitiveContract(
	path string,
	profile ProviderCredentialProfile,
	fields map[ProviderCredentialFieldID]ProviderCredentialField,
) error {
	hasSecret := false
	for _, fieldID := range profile.Fields {
		if fields[fieldID].Kind == ProviderCredentialFieldSecret {
			hasSecret = true
			break
		}
	}
	switch profile.Primitive {
	case ProviderAuthenticationNone:
		if len(profile.Fields) != 0 || len(profile.Placements) != 0 || len(profile.Scopes) != 0 ||
			len(profile.EndpointBindings) != 0 || profile.ProtocolOptions.GoogleDefault != nil ||
			profile.ProtocolOptions.AWSDefault != nil {
			return credentialError(path, profile.ID, "none authentication cannot define credential behavior")
		}
	case ProviderAuthenticationAPIKey, ProviderAuthenticationBearerToken:
		if !hasSecret {
			return credentialError(path+".fields", profile.Fields, "must include a secret field")
		}
		if len(profile.Placements) == 0 {
			return credentialError(path+".placements", nil, "must contain at least one placement")
		}
	case ProviderAuthenticationGoogleDefault, ProviderAuthenticationAzureDefault:
		if !hasSecret {
			return credentialError(path+".fields", profile.Fields, "must include the resolved access-token field")
		}
		if len(profile.Scopes) == 0 {
			return credentialError(path+".scopes", nil, "must contain at least one scope")
		}
		if !hasBearerPlacement(profile.Placements) {
			return credentialError(path+".placements", profile.Placements, "must contain a bearer placement")
		}
	case ProviderAuthenticationAWSDefault:
		if profile.ProtocolOptions.AWSDefault == nil {
			return credentialError(path+".protocol_options.aws_default", nil, "is required for aws-default authentication")
		}
	}
	return nil
}

func hasBearerPlacement(placements []ProviderCredentialPlacement) bool {
	for _, placement := range placements {
		if placement.Kind == ProviderCredentialPlacementHeader && placement.Scheme == ProviderCredentialSchemeBearer {
			return true
		}
	}
	return false
}

func validateCredentialPlane(
	path string,
	plane ProviderCredentialPlane,
	profiles map[ProviderCredentialProfileID]ProviderCredentialProfile,
	referenced map[ProviderCredentialProfileID]struct{},
) error {
	if plane.Required && len(plane.Alternatives) == 0 {
		return credentialError(path+".alternatives", nil, "must contain at least one profile when the plane is required")
	}
	seen := make(map[ProviderCredentialProfileID]struct{}, len(plane.Alternatives))
	for index, profileID := range plane.Alternatives {
		alternativePath := fmt.Sprintf("%s.alternatives[%d]", path, index)
		profile, exists := profiles[profileID]
		if !exists {
			return credentialError(alternativePath, profileID, "does not reference a declared profile")
		}
		if _, exists := seen[profileID]; exists {
			return credentialError(alternativePath, profileID, "must be unique within the plane")
		}
		if profile.Primitive == ProviderAuthenticationNone && (plane.Required || len(plane.Alternatives) != 1) {
			return credentialError(alternativePath, profileID, "none authentication must be the sole alternative on an optional plane")
		}
		seen[profileID] = struct{}{}
		referenced[profileID] = struct{}{}
	}
	return nil
}

func validAuthenticationPrimitive(primitive ProviderAuthenticationPrimitive) bool {
	switch primitive {
	case ProviderAuthenticationNone,
		ProviderAuthenticationAPIKey,
		ProviderAuthenticationBearerToken,
		ProviderAuthenticationGoogleDefault,
		ProviderAuthenticationAzureDefault,
		ProviderAuthenticationAWSDefault:
		return true
	default:
		return false
	}
}

func credentialError(field string, value any, message string) error {
	return providerContractError("provider.credentials."+field, value, message)
}
