//go:build !windows

package runtime

func preparePrivateRuntimeLock(_ string) error { return nil }
