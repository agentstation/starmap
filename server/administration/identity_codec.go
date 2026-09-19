package administration

import (
	"bytes"
	"encoding/json"
	"io"
)

const maximumIdentityRecordBytes = 4 << 20

func decodeIdentities(data []byte, audience string) (identitiesRecord, *identitySnapshot, error) {
	if len(data) == 0 || len(data) > maximumIdentityRecordBytes {
		return identitiesRecord{}, nil, invalidIdentityRecord()
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record identitiesRecord
	if err := decoder.Decode(&record); err != nil {
		return identitiesRecord{}, nil, invalidIdentityRecord()
	}
	if _, err := decoder.Token(); err != io.EOF {
		return identitiesRecord{}, nil, invalidIdentityRecord()
	}
	canonical, err := json.Marshal(record)
	if err != nil || !bytes.Equal(bytes.TrimSpace(data), canonical) {
		return identitiesRecord{}, nil, invalidIdentityRecord()
	}
	snapshot, err := buildIdentitySnapshot(record, audience)
	if err != nil {
		return identitiesRecord{}, nil, err
	}
	return record, snapshot, nil
}
