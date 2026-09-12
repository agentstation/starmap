package workspace

import (
	"io"
	"os"
)

func readWorkspaceRecordBytes(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, invalidReplacement("file")
	}
	file, err := openSnapshotEntry(root, name, info)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, replacementConflict(name, "file changed before the read")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, replacementLimit("file")
	}
	after, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(opened, after) || !after.Mode().IsRegular() || after.Size() != int64(len(data)) || !after.ModTime().Equal(info.ModTime()) {
		return nil, replacementConflict(name, "file changed during the read")
	}
	return data, nil
}
