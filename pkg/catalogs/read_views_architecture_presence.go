package catalogs

import "github.com/agentstation/starmap/pkg/catalogs/authority"

// selectArchitectureClaim selects known facts before unknown architecture claims.
func selectArchitectureClaim(reader Reader, policies authority.Reader, candidates []providerModelCandidate, read func(*ModelArchitecture) (bool, ValuePresence)) (bool, ValuePresence, error) {
	for _, presence := range []ValuePresence{ValueKnown, ValueUnknown} {
		claim, found, err := selectDefinitionValue(reader, policies, "Metadata", candidates, func(model Model) (definitionPresenceClaim[bool], bool) {
			value, state := read(modelArchitecture(model))
			return definitionPresenceClaim[bool]{Value: value, State: state}, state == presence
		})
		if err != nil {
			return false, ValueMissing, err
		}
		if found {
			return claim.Value, claim.State, nil
		}
	}
	return false, ValueMissing, nil
}
