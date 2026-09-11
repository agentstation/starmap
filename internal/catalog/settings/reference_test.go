package settings_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
)

var updateSettingsReference = flag.Bool("update-settings-reference", false, "regenerate the catalog settings reference")

func TestCatalogSettingsReferenceIsCurrent(t *testing.T) {
	descriptors := catalogconfig.Descriptors()
	schema, err := json.MarshalIndent(struct {
		Version  int                        `json:"schema_version"`
		Settings []catalogconfig.Descriptor `json:"settings"`
	}{catalogconfig.SchemaVersion, descriptors}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"docs/catalog-settings-schema.json": append(schema, '\n'),
		"docs/CATALOG_SETTINGS.md":          renderSettingsReference(descriptors),
	}
	for name, expected := range files {
		path := repositoryFile(t, name)
		if *updateSettingsReference {
			if err := os.WriteFile(path, expected, constants.FilePermissions); err != nil {
				t.Fatal(err)
			}
		}
		actual, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actual, expected) {
			t.Fatalf("%s is stale. Run go test ./internal/catalog/settings -run '^TestCatalogSettingsReferenceIsCurrent$' -args -update-settings-reference", name)
		}
	}
}

func renderSettingsReference(descriptors []catalogconfig.Descriptor) []byte {
	result := []byte(settingsReferenceIntroduction)
	for _, d := range descriptors {
		result = fmt.Appendf(result, "\n<a id=\"%s\"></a>\n\n## %s\n\n%s\n\n", d.Anchor, d.Key, d.Description)
		result = fmt.Appendf(result, "| Property | Value |\n|---|---|\n| Environment | `%s` |\n| CLI flag | `--%s value` |\n| YAML key | `%s` |\n| Semantic ID | `%s` |\n| Grammar | `%s` |\n", strings.Join(d.Environment, "`, `"), d.Flag, d.Key, d.ID, d.Type)
		if len(d.AllowedValues) > 0 {
			result = fmt.Appendf(result, "| Accepted names | `%s` |\n", strings.Join(d.AllowedValues, "`, `"))
		}
		if d.DefaultMeaning != "" {
			result = fmt.Appendf(result, "| Default | %s |\n", d.DefaultMeaning)
		} else if d.Default == "" {
			result = append(result, "| Default | Empty |\n"...)
		} else {
			result = fmt.Appendf(result, "| Default | `%s` |\n", d.Default)
		}
		result = fmt.Appendf(result, "| Explicit empty | %t |\n| Explicit zero | %t |\n| Sensitive | %t |\n| Scope | `%s` |\n| Applicability | `%s` |\n| Change class | `%s` |\n| Compatibility | `%s`, schema %d |\n", d.AllowEmpty, d.AllowZero, d.Sensitive, d.Scope, strings.Join(d.Applicability, "`, `"), d.Mutability, d.Compatibility, d.Introduced)
		if d.SourceBinding != "" {
			result = fmt.Appendf(result, "| Source group | `%s` |\n", d.SourceBinding)
		}
	}
	return result
}

const settingsReferenceIntroduction = `# Catalog settings reference

This reference describes the Starmap catalog configuration contract in this source revision.
The public package is ` + "`github.com/agentstation/starmap/pkg/catalogs/config`" + `.
The [descriptor export](catalog-settings-schema.json) contains the same metadata.
Starport adoption remains a separate implementation task.

The descriptors generate this reference and CLI descriptions.
To regenerate both reference files, run:

` + "```sh\ngo test ./internal/catalog/settings -run '^TestCatalogSettingsReferenceIsCurrent$' -args -update-settings-reference\n```" + `

## Value selection

The Starmap command selects each independent value in this order:

1. An explicit command flag.
2. The process environment, including an explicit empty value.
3. Explicit dotenv files, with the last listed file first.
4. The selected configuration file.
5. The runtime default.

YAML uses the flat keys listed below.
Boolean, integer, and string-list values can use native YAML types.
Durations use Go duration syntax, such as ` + "`4h`" + ` or ` + "`30s`" + `.
Unknown keys fail validation.
A valid higher-priority value can replace an invalid lower-priority scalar value.
Malformed files and wrong YAML value types fail before value selection.

All catalog CLI flags take a value, including boolean flags.
For example, ` + "`--catalog-acquisition-enabled=false`" + ` disables automatic acquisition.
An unchanged flag does not override another input.
The parser trims surrounding whitespace and preserves explicit false, zero, and permitted empty values.
An empty value fails unless its descriptor permits it.

## Source and credential boundaries

The source group contains the kind, endpoint, repository, channel, signing workflow, and transport credentials.
A higher-priority source identity replaces the complete lower source group.
Supply the source kind and its required endpoint together.
Lower source credentials do not transfer to the replacement source.
A higher-priority credential can replace a credential without changing the source identity.
An explicit empty credential prevents lower credential fallback.

Poll intervals, freshness thresholds, and acquisition policy keep independent precedence.
The resolver reports selected and ignored origins without credential values.
The Starmap source API key authenticates catalog transport.
The GitHub source token authenticates GitHub access.
Neither is a provider inference key.

## Permission clock configuration

The permission clock source defaults to disabled. Public and embedded catalog use need no native clock profile.
Internal authority permissions require qualified time evidence before they can authorize new work.
Selecting native mode requires explicit cache age, refresh interval, counter drift, and counter uncertainty.
Windows also requires synchronization source age, source drift, and additional source uncertainty.

The refresh interval must be less than half the maximum cache age.
Configuration declares these bounds. It does not qualify the host, time service, or error profile.

Clock settings have node scope and require restart after changes.
Each host supplies its own qualified values. They do not inherit from another product or from shared deployment configuration.
Changing the catalog source does not reset clock settings.

An explicit disabled source clears an earlier host clock selection.
The public parser preserves each setting separately. Application composition validates the complete native profile before runtime startup.

Construction starts no clock query. Runtime startup owns background observations, and runtime shutdown cancels them.
Request admission reads cached evidence without a time-service query.
An unqualified observation invalidates that evidence while catalog diagnostics remain available.
Native clock status errors are local diagnostics and need redaction before public exposure.

## Provider binding declarations

` + "`catalog_provider_bindings`" + ` selects the complete active binding set for connected-runtime acquisition.
Supply YAML as a list of binding objects.
Environment variables and CLI flags accept the same objects as a JSON array.
The value uses the ` + "`sources.ProviderAcquisitionBinding`" + ` wire contract.

Each object declares a schema version, binding ID, revision, provider, scope, API surface, region, and credential profile.
The credential role must be ` + "`catalog_acquisition`" + `.
The declarations contain no credential material and do not prove upstream account ownership.

Schema 2 supports explicit [membership replacement authority](CATALOG_STORE_CONTRACT.md#scope-replacement-and-transport).
Omission grants no replacement authority. Schema 1 remains readable and cannot grant that authority.

An explicit ` + "`[]`" + ` permits no local provider acquisition.
Omission retains legacy unscoped acquisition. Empty text and ` + "`null`" + ` are invalid.
A higher-priority array replaces the entire lower array.
Bindings from separate configuration authorities do not combine.

Replacing the upstream catalog source does not replace this independent binding policy.
Unknown binding fields, invalid declarations, and duplicate binding IDs fail validation.
Changing the binding set requires a new runtime.

The current standalone update command and HTTP update adapter still require binding-policy integration.
Do not use those paths to enforce a scoped acquisition policy.
Starport adoption and complete product qualification remain open.

## Internal authority runtime

The require_authority startup policy keeps metadata available while permission controls new work.
Select the starmap source, its URL, and both source authority and policy IDs.
The runtime accepts only that authority's catalog and disables local acquisition.
A retained catalog does not by itself authorize requests.
Permission checks run independently of catalog checks and the shared acquisition lease.

The runtime stores its highest requirement and finite receipt in catalog-runtime/permission.json beneath the state directory.
This private checkpoint stays uncertain until shutdown finishes and retains every known requirement.
After a crash, metadata remains available, but new work requires a fresh verified receipt.
A completed shutdown permits offline restart while the retained receipt remains valid.

The library host supplies cached clock evidence through WithPermissionClockUncertainty.
Unknown clock validity blocks new work. The callback must use the same time source as WithClock.
The current standalone composition supplies no clock qualification adapter.
Authority receipt issuance and complete deployment qualification remain open.
These settings do not establish a qualified internal-server recipe by themselves.

## Explicit dotenv files

Service configuration does not discover dotenv files in the working directory.
A local operator can name files explicitly:

` + "```sh\nstarmap --env-file .env --env-file .env.local version\n```" + `

Later files replace earlier file values.
The process environment takes precedence over every file, even when its value is empty.
Conflicting file values produce diagnostics with setting names and file paths.
Diagnostics omit both values.
All files must pass private-access checks and parse before any environment change occurs.

Each file permits at most 1 MiB. Private read-only files remain valid.
See [configuration access and recovery](CLI.md#private-configuration-inputs) for platform requirements.

Catalog dotenv values stay in their named file layers.
Other dotenv values enter the process environment only when the process does not already define them.
This supplies provider credentials to the existing acquisition resolver.
Do not place credentials in command arguments or commit them to a repository.

## Legacy migration

` + "`REMOTE_SERVER_URL`" + ` and ` + "`remote_server_url`" + ` imply the Starmap source kind within their own input layer.
Their matching API key aliases are ` + "`REMOTE_SERVER_API_KEY`" + ` and ` + "`remote_server_api_key`" + `.
A canonical source identity replaces the legacy source group within that layer.
The command reports legacy names without their values.
Migrate the complete source group together.

## Library and application responsibilities

` + "`Parse`" + ` accepts canonical names and rejects unknown names.
` + "`CanonicalValues`" + ` converts descriptor keys, semantic IDs, and canonical environment names.
Conflicting aliases fail.
` + "`Load`" + ` reads a caller-supplied lookup and cannot enumerate unknown names.
` + "`Resolve`" + ` accepts ordered layers after the host selects eligible authorities.
These APIs do not read files, inspect the environment, or start network work.

` + "`Config.Options()`" + ` supplies runtime options for the selected values.
Absent values leave defaults with the runtime or hosting application.
` + "`Config.Value()`" + ` reports supplied values and presence, including secret values.
Do not expose that method's output through diagnostics.

Node settings belong to this process.
Deployment settings belong to the selected deployment authority.
A change class describes the required application action.
It does not imply a live settings API in the current command.
The command applies configuration at startup.

Platform roots, primary file selection, storage migration, and Starport shared configuration need their separate implementation and qualification.
This reference does not qualify those features.
`
