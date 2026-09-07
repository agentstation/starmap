package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

func observationID(observation Observation) string {
	var identity strings.Builder
	version := "v2"
	if observation.ProviderBinding != nil {
		version = "v3"
	}
	identity.WriteString("starmap/source-observation/" + version)
	for _, field := range observationIdentityFields(observation) {
		identity.WriteString(strconv.Itoa(len(field)))
		identity.WriteByte(':')
		identity.WriteString(field)
	}
	digest := sha256.Sum256([]byte(identity.String()))
	return "observation:" + version + ":" + hex.EncodeToString(digest[:])
}

func observationIdentityFields(observation Observation) []string {
	capacity := 12 + 3*len(observation.Issues)
	if observation.ProviderBinding != nil {
		capacity += len(observation.ProviderBinding.identityFields())
	}
	fields := make([]string, 0, capacity)
	fields = append(fields,
		string(observation.SourceID),
		observation.ObservedAt.UTC().Format(time.RFC3339Nano),
		string(observation.Revision.Kind), observation.Revision.Value,
		string(observation.Completeness), string(observation.Status),
		observation.EvidenceChecksum,
		observation.Revision.InputName, observation.Revision.InputChecksum,
		strconv.Itoa(observation.Records.Accepted), strconv.Itoa(observation.Records.Rejected),
		strconv.Itoa(len(observation.Issues)),
	)
	for _, issue := range observation.Issues {
		// Diagnostic messages can contain credentials and do not belong in identity.
		fields = append(fields, string(issue.Scope), string(issue.Code), issue.Subject)
	}
	if observation.ProviderBinding != nil {
		bindingFields := observation.ProviderBinding.identityFields()
		fields = append(fields, bindingFields[:]...)
	}
	return fields
}

func matchesLegacyObservationID(observation Observation) bool {
	if observation.ProviderBinding != nil {
		return false
	}
	const prefix = "observation:"
	if !strings.HasPrefix(observation.ID, prefix) || len(observation.ID) != len(prefix)+sha256.Size*2 {
		return false
	}
	// Legacy separators are safe only when no field contains a separator.
	for _, field := range observationIdentityFields(observation) {
		if strings.ContainsRune(field, '\x00') {
			return false
		}
	}
	return observation.ID == legacyObservationID(observation)
}

func legacyObservationID(observation Observation) string {
	var identity strings.Builder
	identity.WriteString(strings.Join([]string{
		string(observation.SourceID),
		observation.ObservedAt.UTC().Format(time.RFC3339Nano),
		string(observation.Revision.Kind),
		observation.Revision.Value,
		string(observation.Completeness),
		string(observation.Status),
		observation.EvidenceChecksum,
	}, "\x00"))
	identity.WriteString("\x00" + observation.Revision.InputName + "\x00" + observation.Revision.InputChecksum)
	if observation.Records.Accepted != 0 || observation.Records.Rejected != 0 {
		identity.WriteString("\x00records:")
		identity.WriteString(strconv.Itoa(observation.Records.Accepted))
		identity.WriteByte(':')
		identity.WriteString(strconv.Itoa(observation.Records.Rejected))
	}
	for _, issue := range observation.Issues {
		// Human-readable diagnostics can contain transport details or secrets and
		// are deliberately excluded from stable identity and long-term evidence.
		identity.WriteString("\x00" + string(issue.Scope) + "\x00" + string(issue.Code) + "\x00" + issue.Subject)
	}
	digest := sha256.Sum256([]byte(identity.String()))
	return "observation:" + hex.EncodeToString(digest[:])
}
