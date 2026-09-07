package runtime

import "os"

func preparePrivateRuntimeLock(directory string) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	file, err := createPrivateRuntimeFile(root, directoryLockName)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return file.Close()
}
