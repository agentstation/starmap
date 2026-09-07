//go:build darwin || linux

package workspace

import (
	"os"
	"syscall"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

func copyNativeAccess(source, destination *os.File) error {
	before, err := source.Stat()
	if err != nil {
		return err
	}
	current, err := destination.Stat()
	if err != nil {
		return err
	}
	want, ok := before.Sys().(*syscall.Stat_t)
	got, present := current.Sys().(*syscall.Stat_t)
	if !ok || !present {
		return &errors.ConfigError{Component: "workspace access", Message: "native ownership is unavailable"}
	}
	if want.Uid != got.Uid || want.Gid != got.Gid {
		if err := destination.Chown(int(want.Uid), int(want.Gid)); err != nil {
			return err
		}
	}
	if err := destination.Chmod(before.Mode() & workspaceAccessMode); err != nil {
		return err
	}
	acl, err := nativeEntryACL(source)
	if err != nil {
		return err
	}
	return setNativeEntryACL(destination, acl)
}

func openStagedDirectory(root *os.Root, name string) (*os.File, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	return openSnapshotEntry(root, name, info)
}

func createStagedFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, fileMode)
}

func privateStageAccess(file *os.File) error {
	if err := setNativeEntryACL(file, nil); err != nil {
		return err
	}
	return file.Chmod(privatefiles.DirectoryMode)
}
