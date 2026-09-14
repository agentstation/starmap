//go:build darwin || linux

package privatefiles

import (
	"encoding/json"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/agentstation/starmap/pkg/errors"
)

func nativePublicationAccess(file *os.File) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, &errors.ConfigError{Component: "private record access", Message: "native ownership is unavailable"}
	}
	acl, err := nativePublicationACL(file)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		UID  uint32 `json:"uid"`
		GID  uint32 `json:"gid"`
		Mode uint32 `json:"mode"`
		ACL  []byte `json:"acl"`
	}{stat.Uid, stat.Gid, uint32(info.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)), acl})
}

func openPublicationEntry(root *os.Root, name string, info fs.FileInfo) (*os.File, error) {
	flags := os.O_RDONLY | unix.O_NONBLOCK | unix.O_NOFOLLOW
	if info.IsDir() {
		flags |= unix.O_DIRECTORY
	}
	return root.OpenFile(name, flags, 0)
}
