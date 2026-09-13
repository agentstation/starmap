//go:build !windows

package filemutation

func nativeRenameRefusal(error) bool { return false }
