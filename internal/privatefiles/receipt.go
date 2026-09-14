package privatefiles

import "os"

// EntryReceipt binds a directory or regular file to its native identity and access policy.
// Directory receipts do not bind child contents or modification times.
type EntryReceipt = publicationEntry

// RecordReceipt also binds regular-file contents, size, mode, and modification time.
type RecordReceipt = publicationRecord

// CaptureEntry records one private direct child under an open root.
func CaptureEntry(root *os.Root, name string, directory bool) (EntryReceipt, error) {
	if name != "." {
		if err := childName(name); err != nil {
			return EntryReceipt{}, err
		}
	}
	return publicationEntryOf(root, name, directory)
}

// CaptureRecord records one bounded private regular file under an open root.
func CaptureRecord(root *os.Root, name string, limit int64) (RecordReceipt, error) {
	if err := childName(name); err != nil {
		return RecordReceipt{}, err
	}
	return publicationRecordOf(root, name, limit)
}

// CheckEntry rejects a different native identity or access policy.
func CheckEntry(root *os.Root, name string, directory bool, want EntryReceipt) error {
	got, err := CaptureEntry(root, name, directory)
	if err != nil {
		return err
	}
	if got != want {
		return changed(name)
	}
	return nil
}

// CheckRecord rejects changed ownership, metadata, identity, or contents.
func CheckRecord(root *os.Root, name string, want RecordReceipt) error {
	got, err := CaptureRecord(root, name, want.Size)
	if err != nil {
		return err
	}
	if got != want {
		return changed(name)
	}
	return nil
}
