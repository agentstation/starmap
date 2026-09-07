package privatefiles

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"

	aclpolicy "github.com/agentstation/starmap/internal/runtimeacl/windows"
	"github.com/agentstation/starmap/pkg/errors"
)

const maxWindowsAncestorLinks = 255

// ValidateAncestors checks Windows ancestor ownership, DACLs, and selected symlink routes.
// It does not create paths or change native permissions.
func ValidateAncestors(path string) error {
	account, err := windowsProcessSID()
	if err != nil {
		return err
	}
	full, err := windowsAncestorAbsolute(path)
	if err != nil {
		return err
	}
	current := filepath.VolumeName(full) + `\`
	pending := strings.Split(full[len(current):], `\`)
	if err := validateWindowsAncestorPath(current, account); err != nil {
		return err
	}
	links := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			current = filepath.Dir(current)
			continue
		}
		if strings.ContainsAny(part, ":\x00") {
			return &errors.ValidationError{Field: "private.ancestor", Message: "alternate streams and NUL are not supported in private paths"}
		}
		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			links++
			if links > maxWindowsAncestorLinks {
				return &errors.ValidationError{Field: "private.ancestor", Message: "selected route exceeds the supported symlink limit"}
			}
			if err := validateWindowsAncestorEntry(next, info, account); err != nil {
				return err
			}
			target, err := os.Readlink(next)
			if err != nil {
				return err
			}
			target = strings.ReplaceAll(target, "/", `\`)
			if filepath.IsAbs(target) {
				absolute, err := windowsAncestorAbsolute(target)
				if err != nil {
					return err
				}
				current = filepath.VolumeName(absolute) + `\`
				if err := validateWindowsAncestorPath(current, account); err != nil {
					return err
				}
				target = absolute[len(current):]
			}
			pending = append(strings.Split(target, `\`), pending...)
			continue
		}
		if len(pending) == 0 {
			return nil
		}
		if !info.IsDir() {
			return &errors.ValidationError{Field: "private.ancestor", Value: next, Message: "must resolve to a real directory"}
		}
		if err := validateWindowsAncestorEntry(next, info, account); err != nil {
			return err
		}
		current = next
	}
	return nil
}

func windowsAncestorAbsolute(path string) (string, error) {
	path = strings.ReplaceAll(path, "/", `\`)
	var err error
	path, err = windowsAncestorNamespace(path)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(path) {
		volume := filepath.VolumeName(path)
		if len(path) == len(volume) {
			return path + `\`, nil
		}
		return path, nil
	}
	volume := filepath.VolumeName(path)
	if volume != "" {
		base, err := filepath.Abs(volume + ".")
		if err != nil {
			return "", err
		}
		return strings.TrimRight(base, `\`) + `\` + path[len(volume):], nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(path, `\`) {
		return filepath.VolumeName(cwd) + path, nil
	}
	return strings.TrimRight(cwd, `\`) + `\` + path, nil
}

func validateWindowsAncestorPath(path, account string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &errors.ValidationError{Field: "private.ancestor", Value: path, Message: "must be a directory"}
	}
	return validateWindowsAncestorEntry(path, info, account)
}

func validateWindowsAncestorEntry(path string, expected fs.FileInfo, account string) error {
	native := path
	if !strings.HasPrefix(path, `\\?\`) {
		native = `\\?\` + path
		if strings.HasPrefix(path, `\\`) {
			native = `\\?\UNC\` + strings.TrimPrefix(path, `\\`)
		}
	}
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return errors.WrapIO("inspect Windows ancestor", path, err)
	}
	file := os.NewFile(uintptr(handle), path)
	defer func() { _ = file.Close() }()
	actual, err := file.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(expected, actual) || !os.SameFile(expected, current) {
		return changed(path)
	}
	var attributes windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &attributes); err != nil {
		return err
	}
	if attributes.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 && expected.Mode()&os.ModeSymlink == 0 {
		return &errors.ValidationError{Field: "private.ancestor", Value: path, Message: "unsupported reparse point in private path"}
	}
	sd, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return errors.WrapIO("read Windows ancestor security", path, err)
	}
	descriptor, err := decodeWindowsSecurityDescriptor(sd, "private.ancestor")
	if err != nil {
		return err
	}
	if err := aclpolicy.ValidateAncestor(account, descriptor.Owner, descriptor.Present, descriptor.Null, descriptor.Entries, "private.ancestor"); err != nil {
		return errors.WrapIO("validate Windows ancestor", path, err)
	}
	return nil
}

func windowsAncestorNamespace(path string) (string, error) {
	if !strings.HasPrefix(path, `\\?\`) && !strings.HasPrefix(path, `\\.\`) && !strings.HasPrefix(path, `\??\`) {
		return path, nil
	}
	rest := path[4:]
	if len(rest) >= 4 && strings.EqualFold(rest[:4], `UNC\`) {
		return `\\` + rest[4:], nil
	}
	if len(rest) >= 3 && ((rest[0] >= 'a' && rest[0] <= 'z') || (rest[0] >= 'A' && rest[0] <= 'Z')) && rest[1] == ':' && rest[2] == '\\' {
		return rest, nil
	}
	volume, _, _ := strings.Cut(rest, `\`)
	if len(volume) > 6 && strings.EqualFold(volume[:6], "Volume") {
		if _, err := windows.GUIDFromString(volume[6:]); err == nil {
			return `\\?\` + rest, nil
		}
	}
	return "", &errors.ValidationError{Field: "private.ancestor", Message: "private paths require a drive, UNC share, or volume GUID filesystem path"}
}
