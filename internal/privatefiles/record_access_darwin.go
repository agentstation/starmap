package privatefiles

import "os"

func nativePublicationACL(file *os.File) ([]byte, error) {
	api, err := nativeDarwinACL()
	if err != nil {
		return nil, err
	}
	buffer, err := darwinACLBuffer(api, file, "private record access")
	if err != nil {
		return nil, err
	}
	if _, err := darwinACLGrants(buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}
