package privatefiles

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func publicationAccess(file *os.File) (string, error) {
	data, err := nativePublicationAccess(file)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
