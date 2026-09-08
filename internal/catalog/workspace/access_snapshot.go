package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func entryAccessDigest(file *os.File) (string, error) {
	data, err := nativeEntryAccess(file)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
