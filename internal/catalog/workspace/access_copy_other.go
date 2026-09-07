//go:build !darwin && !linux && !windows

package workspace

import "os"

func copyNativeAccess(source, destination *os.File) error {
	_, err := nativeEntryAccess(source)
	return err
}
func openStagedDirectory(root *os.Root, name string) (*os.File, error) { return root.Open(name) }
func createStagedFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, fileMode)
}
func privateStageAccess(file *os.File) error { _, err := nativeEntryAccess(file); return err }
