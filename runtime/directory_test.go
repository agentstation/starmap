package runtime

import (
	"bufio"
	"context"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestPersistentRuntimeRefusesConcurrentDirectoryOwner(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	first := openTestRuntime(t, WithStateDirectory(directory), WithListenAddress("127.0.0.1:8080"))
	seedPath := filepath.Join(directory, instanceSeedFileName)
	before, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithListenAddress("127.0.0.1:9090"))
	if second != nil {
		_ = second.Close()
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("second runtime error = %v, want ownership conflict", err)
	}
	after, err := os.ReadFile(seedPath)
	if err != nil || string(after) != string(before) {
		t.Fatal("refused owner changed the instance seed")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	replacement := openTestRuntime(t, WithStateDirectory(directory))
	if replacement.Status().InstanceIdentity == "" {
		t.Fatal("replacement did not acquire the runtime directory")
	}
}

func TestClosedRuntimeRefusesNewAcquisition(t *testing.T) {
	t.Parallel()
	acquirer := &stubAcquirer{}
	connected := openTestRuntime(t, WithAcquirer(acquirer))
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := connected.Sync(context.Background()); err == nil {
		t.Fatal("closed runtime accepted a new acquisition")
	}
	if acquirer.callCount() != 0 {
		t.Fatal("closed runtime reached the acquirer")
	}
}

func TestRuntimeDirectoryLockSurvivesProcessInterruption(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestRuntimeDirectoryLockChild$")
	command.Env = append(os.Environ(), "STARMAP_DIRECTORY_LOCK_TEST="+directory)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		_ = command.Process.Kill()
		if command.ProcessState == nil {
			_ = command.Wait()
		}
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("child readiness = %q, %v", line, err)
	}
	lock, err := acquireDirectory(t.Context(), directory)
	if lock != nil {
		_ = lock.Close()
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("live child lock = %v, want conflict", err)
	}
	if _, err := input.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	_ = input.Close()
	err = command.Wait()
	var exited *exec.ExitError
	if !stderrors.As(err, &exited) || exited.ExitCode() != 86 {
		t.Fatalf("child exit = %v", err)
	}
	recovered, err := acquireDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, directoryLockName)); err != nil {
		t.Fatal("release removed the shared lock file")
	}
}

func TestRuntimeDirectoryLockChild(t *testing.T) {
	directory := os.Getenv("STARMAP_DIRECTORY_LOCK_TEST")
	if directory == "" {
		return
	}
	lock, err := acquireDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil {
		t.Fatal("child acquired no lock")
	}
	for range 3 {
		goruntime.GC()
	}
	if _, err := os.Stdout.WriteString("locked\n"); err != nil {
		t.Fatal(err)
	}
	var signal [1]byte
	if _, err := os.Stdin.Read(signal[:]); err != nil {
		t.Fatal(err)
	}
	// Retain the lock descriptor until the parent permits process exit.
	goruntime.KeepAlive(lock)
	os.Exit(86)
}

func TestRuntimeDirectoryLockRejectsSymlinksAndCancellation(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "unused")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := acquireDirectory(ctx, directory); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancelled ownership = %v", err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("cancelled ownership created a directory")
	}
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	if err := os.WriteFile(marker, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	directory = privateRuntimeDirectory(t)
	if err := os.Symlink(marker, filepath.Join(directory, directoryLockName)); err != nil {
		t.Skipf("native symlink creation unavailable: %v", err)
	}
	if lock, err := acquireDirectory(t.Context(), directory); err == nil {
		_ = lock.Close()
		t.Fatal("ownership followed a symlink")
	}
	after, err := os.ReadFile(marker)
	if err != nil || string(after) != "preserve" {
		t.Fatal("symlink refusal changed another file")
	}
}

func TestRuntimeDirectoryRemainsOwnedUntilCallerRunStops(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	release := make(chan struct{})
	acquirer := &stubAcquirer{entered: make(chan struct{})}
	acquirer.observe = func(context.Context) { <-release }
	connected := openTestRuntime(t, WithStateDirectory(directory), WithAcquirer(acquirer))
	finished := make(chan struct{})
	go func() { _, _ = connected.Sync(context.Background()); close(finished) }()
	select {
	case <-acquirer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("acquisition did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- connected.Close() }()
	<-connected.ctx.Done()
	lock, err := acquireDirectory(t.Context(), directory)
	if lock != nil {
		_ = lock.Close()
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Errorf("closing runtime lost its lock: %v", err)
	}
	close(release)
	<-finished
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	replacement, err := acquireDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFailedRuntimeOpenReleasesDirectory(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	if err := os.WriteFile(filepath.Join(directory, layerDirectoryName), []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	connected, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false))
	if connected != nil {
		_ = connected.Close()
	}
	if err == nil {
		t.Fatal("runtime opened with an invalid layer directory")
	}
	lock, err := acquireDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseTimeoutKeepsRuntimeDirectoryOwned(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	release := make(chan struct{})
	var releaseOnce sync.Once
	stop := func() { releaseOnce.Do(func() { close(release) }) }
	defer stop()
	acquirer := &stubAcquirer{entered: make(chan struct{})}
	acquirer.observe = func(context.Context) { <-release }
	connected, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0), WithAcquirer(acquirer))
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	go func() { _, _ = connected.Sync(context.Background()); close(finished) }()
	select {
	case <-acquirer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("acquisition did not start")
	}
	var timeout *errors.TimeoutError
	if err := connected.Close(); !stderrors.As(err, &timeout) {
		t.Fatalf("close = %v, want timeout", err)
	}
	held, err := acquireDirectory(t.Context(), directory)
	if held != nil {
		_ = held.Close()
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("timed-out runtime lost ownership: %v", err)
	}
	stop()
	<-finished
	requireDirectoryRelease(t, directory)
}

type blockedReleaseStore struct {
	*stubLeaseStore
	entered chan struct{}
	release chan struct{}
}

func (s *blockedReleaseStore) Release(ctx context.Context, lease Lease) error {
	close(s.entered)
	<-s.release
	return s.stubLeaseStore.Release(ctx, lease)
}

func TestRuntimeDirectoryOwnershipIncludesLeaseRelease(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	leases := &blockedReleaseStore{stubLeaseStore: &stubLeaseStore{}, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(leases.release) }) }
	defer release()
	connected, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0), WithLeaseStore(leases))
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- connected.Close() }()
	select {
	case <-leases.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("lease release did not start")
	}
	lock, err := acquireDirectory(t.Context(), directory)
	if lock != nil {
		_ = lock.Close()
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("runtime lost directory ownership before lease release: %v", err)
	}
	select {
	case err := <-closed:
		var timeout *errors.TimeoutError
		if !stderrors.As(err, &timeout) {
			t.Fatalf("close = %v, want bounded timeout", err)
		}
	case <-time.After(closeJoinTimeout + time.Second):
		t.Fatal("lease release exceeded the close bound")
	}
	release()
	requireDirectoryRelease(t, directory)
}

func requireDirectoryRelease(t *testing.T, directory string) {
	t.Helper()
	var conflict *errors.ConflictError
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	retry := time.NewTicker(10 * time.Millisecond)
	defer retry.Stop()
	for {
		lock, err := acquireDirectory(t.Context(), directory)
		if err == nil {
			if err := lock.Close(); err != nil {
				t.Fatal(err)
			}
			return
		}
		if !stderrors.As(err, &conflict) {
			t.Fatal(err)
		}
		select {
		case <-deadline.C:
			t.Fatal("finished runtime retained directory ownership")
		case <-retry.C:
		}
	}
}
