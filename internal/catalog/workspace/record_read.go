package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

func readWorkspaceRecordBytes(root *os.Root, name string, limit int64) ([]byte, error) {
	data, _, err := readWorkspaceRecord(root, name, limit)
	return data, err
}

func readWorkspaceRecord(root *os.Root, name string, limit int64) ([]byte, workspaceRecordState, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, workspaceRecordState{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, workspaceRecordState{}, invalidReplacement("file")
	}
	file, err := openSnapshotEntry(root, name, info)
	if err != nil {
		return nil, workspaceRecordState{}, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, workspaceRecordState{}, err
	}
	if !os.SameFile(info, opened) || opened.Mode() != info.Mode() || opened.Size() != info.Size() || !opened.ModTime().Equal(info.ModTime()) {
		return nil, workspaceRecordState{}, replacementConflict(name, "file changed before the read")
	}
	identity, err := entryIdentity(file)
	if err != nil {
		return nil, workspaceRecordState{}, err
	}
	access, err := entryAccessDigest(file)
	if err != nil {
		return nil, workspaceRecordState{}, err
	}
	data := []byte{}
	// Empty lock files need metadata validation without a Windows locked-region read.
	if opened.Size() != 0 {
		data, err = io.ReadAll(io.LimitReader(file, limit+1))
		if err != nil {
			return nil, workspaceRecordState{}, err
		}
	}
	if int64(len(data)) > limit {
		return nil, workspaceRecordState{}, replacementLimit("file")
	}
	after, err := root.Lstat(name)
	if err != nil {
		return nil, workspaceRecordState{}, err
	}
	if !os.SameFile(opened, after) || !after.Mode().IsRegular() || after.Mode() != info.Mode() || after.Size() != info.Size() || after.Size() != int64(len(data)) || !after.ModTime().Equal(info.ModTime()) {
		return nil, workspaceRecordState{}, replacementConflict(name, "file changed during the read")
	}
	currentAccess, err := entryAccessDigest(file)
	if err != nil {
		return nil, workspaceRecordState{}, err
	}
	if currentAccess != access {
		return nil, workspaceRecordState{}, replacementConflict(name, "access changed during the read")
	}
	digest := sha256.Sum256(data)
	state := workspaceRecordState{
		identity: identity,
		entry: treeEntry{Path: name, Mode: uint32(info.Mode() & workspaceAccessMode), Size: int64(len(data)),
			SHA256: hex.EncodeToString(digest[:]), AccessSHA256: access},
	}
	return data, state, nil
}
