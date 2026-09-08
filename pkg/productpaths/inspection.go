package productpaths

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// DefaultInspectionEntries bounds the default metadata scan, including unmatched entries.
const DefaultInspectionEntries = 10000

// MaximumInspectionEntries caps an explicit metadata scan budget.
const MaximumInspectionEntries = 100000

// FileInspection contains bounded metadata observations, not a readiness or access verdict.
type FileInspection struct {
	ObservedAt   time.Time         `json:"observed_at" yaml:"observed_at"`
	Complete     bool              `json:"complete" yaml:"complete"`
	EntryLimit   int               `json:"entry_limit" yaml:"entry_limit"`
	Examined     int               `json:"examined" yaml:"examined"`
	Observations []FileObservation `json:"observations" yaml:"observations"`
	Limitations  []string          `json:"limitations" yaml:"limitations"`
}

// FileObservation describes one selected location or matching managed artifact.
type FileObservation struct {
	ID              string           `json:"id" yaml:"id"`
	Path            string           `json:"path" yaml:"path"`
	State           string           `json:"state" yaml:"state"`
	Kind            string           `json:"kind,omitempty" yaml:"kind,omitempty"`
	Mode            string           `json:"mode,omitempty" yaml:"mode,omitempty"`
	PermissionScope string           `json:"permission_scope,omitempty" yaml:"permission_scope,omitempty"`
	Owner           *FileOwner       `json:"owner,omitempty" yaml:"owner,omitempty"`
	WindowsSecurity *WindowsSecurity `json:"windows_security,omitempty" yaml:"windows_security,omitempty"`
	OwnerBinding    string           `json:"owner_binding,omitempty" yaml:"owner_binding,omitempty"`
	Reason          string           `json:"reason,omitempty" yaml:"reason,omitempty"`
	AccessPolicy    string           `json:"access_policy,omitempty" yaml:"access_policy,omitempty"`
	AccessStatus    string           `json:"access_status,omitempty" yaml:"access_status,omitempty"`
	AccessReason    string           `json:"access_reason,omitempty" yaml:"access_reason,omitempty"`
}

// FileOwner reports filesystem ownership without asserting runtime or fleet ownership.
type FileOwner struct {
	UID                  uint32 `json:"uid" yaml:"uid"`
	GID                  uint32 `json:"gid" yaml:"gid"`
	MatchesEffectiveUser bool   `json:"matches_effective_user" yaml:"matches_effective_user"`
}

// InspectManifest reads metadata and bounded directory entries without reading file contents.
// It skips observed symbolic-link targets and makes no file writes or runtime lock attempts.
func InspectManifest(ctx context.Context, manifest FileManifest, limit int) (FileInspection, error) {
	if ctx == nil {
		return FileInspection{}, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if limit == 0 {
		limit = DefaultInspectionEntries
	}
	if limit < 1 || limit > MaximumInspectionEntries {
		return FileInspection{}, &errors.ValidationError{Field: "inspection.entry_limit", Message: "must be between 1 and 100000"}
	}
	entries, err := inspectionEntries(manifest)
	if err != nil {
		return FileInspection{}, err
	}
	report := FileInspection{ObservedAt: time.Now().UTC(), Complete: true, EntryLimit: limit,
		Limitations: []string{"Metadata observations are not an atomic snapshot.", "Mode bits and DACL policy checks do not establish effective access or ancestor safety.", "Filesystem ownership does not prove runtime ownership or fleet fencing.", "Observed symbolic links are not followed. File contents are not inspected."}}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			report.Complete = false
			return report, err
		}
		if !report.consume() {
			break
		}
		if entry.Availability != "available" {
			report.Observations = append(report.Observations, FileObservation{ID: entry.ID, Path: entry.Location.Path, State: "not-applicable", Reason: entry.Availability})
			continue
		}
		info, statErr := inspectionLstat(entry.Location.Path)
		report.observe(entry.ID, entry.Location.Path, info, statErr)
		if statErr != nil || !info.IsDir() || len(entry.Patterns) == 0 {
			continue
		}
		if err := report.scan(ctx, entry, info); err != nil {
			report.Complete = false
			return report, err
		}
	}
	assessManifestAccess(manifest, &report)
	return report, nil
}

func inspectionEntries(manifest FileManifest) ([]FileEntry, error) {
	entries := make([]FileEntry, 0, len(manifest.Roots)+len(manifest.Files))
	for _, role := range []Root{Config, Data, State, Cache} {
		if location, ok := manifest.Roots[role]; ok {
			entries = append(entries, FileEntry{ID: "root/" + string(role), Location: location, Availability: "available"})
		}
	}
	entries = append(entries, manifest.Files...)
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.ID == "" || seen[entry.ID] || !slices.Contains([]string{"available", "disabled", "planned"}, entry.Availability) {
			return nil, &errors.ValidationError{Field: "inspection.manifest", Message: "requires unique file roles and known availability"}
		}
		seen[entry.ID] = true
		if entry.Availability == "available" && (!filepath.IsAbs(entry.Location.Path) || strings.ContainsRune(entry.Location.Path, '\x00')) {
			return nil, &errors.ValidationError{Field: "inspection.path", Message: "must be an absolute path without NUL"}
		}
		for _, pattern := range entry.Patterns {
			parts := strings.Split(pattern, "/")
			for index, part := range parts {
				_, err := path.Match(part, "")
				if err != nil || part == "" || part == "." || part == ".." || strings.ContainsRune(part, '\x00') || part == "**" && index != len(parts)-1 {
					return nil, &errors.ValidationError{Field: "inspection.pattern", Message: "must use relative slash-separated patterns with double star only at the end"}
				}
			}
		}
	}
	return entries, nil
}

func (report *FileInspection) consume() bool {
	if report.Examined == report.EntryLimit {
		report.Complete = false
		return false
	}
	report.Examined++
	return true
}

func (report *FileInspection) observe(id, location string, info fs.FileInfo, err error) {
	item := FileObservation{ID: id, Path: location, State: "present"}
	if err != nil {
		item.State, item.Reason = "unavailable", "metadata-error"
		if os.IsNotExist(err) {
			item.State, item.Reason = "absent", ""
		} else {
			report.Complete = false
			if os.IsPermission(err) {
				item.Reason = "permission-denied"
			}
		}
	} else {
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			item.Kind = "symlink"
		case info.IsDir():
			item.Kind = "directory"
		case info.Mode().IsRegular():
			item.Kind = "file"
		default:
			item.Kind = "special"
		}
		inspectPermissions(&item, info)
	}
	report.Observations = append(report.Observations, item)
}

func (report *FileInspection) scan(ctx context.Context, entry FileEntry, expected fs.FileInfo) error {
	root, err := os.OpenRoot(entry.Location.Path)
	if err != nil {
		report.observe(entry.ID, entry.Location.Path, nil, err)
		return nil
	}
	defer func() { _ = root.Close() }()
	current, err := root.Stat(".")
	selected, selectedErr := inspectionLstat(entry.Location.Path)
	if err != nil || selectedErr != nil || selected.Mode()&os.ModeSymlink != 0 || !os.SameFile(current, expected) || !os.SameFile(selected, expected) {
		report.changed(entry.ID, entry.Location.Path)
		return nil
	}
	queue := []string{"."}
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		directory := queue[0]
		queue = queue[1:]
		children, err := report.scanDirectory(ctx, root, entry, directory)
		if err != nil {
			return err
		}
		queue = append(queue, children...)
		if report.Examined == report.EntryLimit && len(queue) > 0 {
			report.Complete = false
			break
		}
	}
	return nil
}

func (report *FileInspection) scanDirectory(ctx context.Context, root *os.Root, entry FileEntry, directory string) ([]string, error) {
	location := filepath.Join(entry.Location.Path, directory)
	expected, err := root.Lstat(directory)
	if err != nil || !expected.IsDir() {
		report.changed(entry.ID, location)
		return nil, nil
	}
	file, err := openInspectionDirectory(root, directory)
	if err != nil {
		report.observe(entry.ID, location, nil, err)
		return nil, nil
	}
	defer func() { _ = file.Close() }()
	actual, err := file.Stat()
	selected, selectedErr := root.Lstat(directory)
	if err != nil || selectedErr != nil || !selected.IsDir() || !os.SameFile(expected, actual) || !os.SameFile(expected, selected) {
		report.changed(entry.ID, location)
		return nil, nil
	}
	var children []string
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		remaining := report.EntryLimit - report.Examined
		if remaining == 0 {
			report.Complete = false
			return children, nil
		}
		batch, readErr := file.ReadDir(min(128, remaining))
		for _, child := range batch {
			if !report.consume() {
				return children, nil
			}
			relative := filepath.Join(directory, child.Name())
			match, descend := matchInspectionPatterns(entry.Patterns, filepath.ToSlash(relative))
			if !match && !descend {
				continue
			}
			info, err := root.Lstat(relative)
			report.observe(entry.ID, filepath.Join(entry.Location.Path, relative), info, err)
			if err == nil && info.IsDir() && descend {
				children = append(children, relative)
			}
		}
		if readErr == io.EOF {
			return children, nil
		}
		if readErr != nil {
			report.observe(entry.ID, location, nil, readErr)
			return children, nil
		}
	}
}

func (report *FileInspection) changed(id, location string) {
	report.Complete = false
	report.Observations = append(report.Observations, FileObservation{ID: id, Path: location, State: "unavailable", Reason: "changed-during-inspection"})
}

func matchInspectionPatterns(patterns []string, relative string) (match, descend bool) {
	names := strings.Split(relative, "/")
	for _, pattern := range patterns {
		parts := strings.Split(pattern, "/")
		compatible := true
		for index, name := range names {
			if index >= len(parts) {
				compatible = false
				break
			}
			if parts[index] == "**" {
				match, descend = true, true
				break
			}
			ok, _ := path.Match(parts[index], name)
			if !ok {
				compatible = false
				break
			}
		}
		if compatible {
			match = match || len(names) == len(parts)
			descend = descend || len(names) < len(parts)
		}
	}
	return match, descend
}
