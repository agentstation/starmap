package valkey

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	client "github.com/valkey-io/valkey-go"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func testBackend(t *testing.T) (*Backend, client.Client) {
	t.Helper()
	address := os.Getenv("STARMAP_TEST_VALKEY_ADDRESS")
	if address == "" {
		t.Skip("STARMAP_TEST_VALKEY_ADDRESS selects the real coordination service")
	}
	connection, err := client.NewClient(client.ClientOption{
		InitAddress: []string{address}, DisableCache: true, DisableRetry: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Close)
	backend, err := New(connection, Config{Prefix: "starmap-coordination-test:" + t.Name(), MaxObjectBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	return backend, connection
}

func TestCoordinationBackendConditionalRecords(t *testing.T) {
	backend, connection := testBackend(t)
	key := strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := backend.GetCurrent(t.Context(), key); !errors.IsNotFound(err) {
		t.Fatalf("absent coordination record: %v", err)
	}
	original := []byte{'a', 0, 0xff, 'z'}
	first, err := backend.Put(t.Context(), key, original, storage.ObjectPutCondition{IfAbsent: true})
	if err != nil || !bytes.Equal(first.Data, original) || first.Version == "" {
		t.Fatalf("initial record: %+v, %v", first, err)
	}
	first.Data[0] = '!'
	read, err := backend.Get(t.Context(), key)
	if err != nil || !bytes.Equal(read.Data, original) || read.Version != first.Version {
		t.Fatalf("stored bytes changed through a returned copy: %+v, %v", read, err)
	}
	for _, condition := range []storage.ObjectPutCondition{{IfAbsent: true}, {IfVersion: "wrong"}} {
		if _, err := backend.Put(t.Context(), key, []byte("refused"), condition); !errors.IsConflict(err) {
			t.Fatalf("conflicting update: %v", err)
		}
	}
	second, err := backend.Put(t.Context(), key, original, storage.ObjectPutCondition{IfVersion: first.Version})
	if err != nil || second.Version == first.Version {
		t.Fatalf("identical bytes reused the old conditional version: %+v, %v", second, err)
	}
	if _, err := backend.Put(t.Context(), key, []byte("stale"), storage.ObjectPutCondition{IfVersion: first.Version}); !errors.IsConflict(err) {
		t.Fatalf("stale writer replaced the new record: %v", err)
	}
	if ttl, err := connection.Do(t.Context(), connection.B().Pttl().Key(backend.key(key)).Build()).AsInt64(); err != nil || ttl != -1 {
		t.Fatalf("coordination record expires: ttl=%d error=%v", ttl, err)
	}
}

func TestCoordinationBackendConcurrentCAS(t *testing.T) {
	backend, _ := testBackend(t)
	other, _ := testBackend(t)
	key := strconv.FormatInt(time.Now().UnixNano(), 10)
	first, err := backend.Put(t.Context(), key, []byte("before"), storage.ObjectPutCondition{IfAbsent: true})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 16)
	var workers sync.WaitGroup
	for index := range 16 {
		workers.Go(func() {
			selected := backend
			if index%2 != 0 {
				selected = other
			}
			_, err := selected.Put(t.Context(), key, []byte(strconv.Itoa(index)), storage.ObjectPutCondition{IfVersion: first.Version})
			results <- err
		})
	}
	workers.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.IsConflict(err):
			conflicted++
		default:
			t.Fatalf("concurrent update: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 15 {
		t.Fatalf("conditional publication admitted %d writers and refused %d", succeeded, conflicted)
	}
}

func TestCoordinationBackendRefusesInvalidAndExpiredRecords(t *testing.T) {
	for _, name := range []string{"type", "shape", "version", "schema", "size", "expiration"} {
		t.Run(name, func(t *testing.T) {
			backend, connection := testBackend(t)
			key := strconv.FormatInt(time.Now().UnixNano(), 10)
			first, err := backend.Put(t.Context(), key, []byte("retained"), storage.ObjectPutCondition{IfAbsent: true})
			if err != nil {
				t.Fatal(err)
			}
			stored := backend.key(key)
			var command client.Completed
			switch name {
			case "type":
				command = connection.B().Set().Key(stored).Value("operator record").Build()
			case "shape":
				command = connection.B().Hset().Key(stored).FieldValue().FieldValue("extra", "preserve").Build()
			case "version":
				command = connection.B().Hset().Key(stored).FieldValue().FieldValue("version", "").Build()
			case "schema":
				command = connection.B().Hset().Key(stored).FieldValue().FieldValue("schema", "2").Build()
			case "size":
				command = connection.B().Hset().Key(stored).FieldValue().FieldValue("data", string(make([]byte, 1025))).Build()
			case "expiration":
				command = connection.B().Pexpire().Key(stored).Milliseconds(60000).Build()
			}
			if err := connection.Do(t.Context(), command).Error(); err != nil {
				t.Fatal(err)
			}
			before, err := connection.Do(t.Context(), connection.B().Dump().Key(stored).Build()).AsBytes()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := backend.GetCurrent(t.Context(), key); !errors.IsValidationError(err) {
				t.Fatalf("invalid record supplied current coordination: %v", err)
			}
			if _, err := backend.Put(t.Context(), key, []byte("replacement"), storage.ObjectPutCondition{IfVersion: first.Version}); !errors.IsValidationError(err) {
				t.Fatalf("invalid record allowed replacement: %v", err)
			}
			after, err := connection.Do(t.Context(), connection.B().Dump().Key(stored).Build()).AsBytes()
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("refusal changed stored bytes: %v", err)
			}
		})
	}
}

func TestCoordinationBackendRefusesCancellationAndUnconditionalWrites(t *testing.T) {
	backend, _ := testBackend(t)
	key := strconv.FormatInt(time.Now().UnixNano(), 10)
	for _, condition := range []storage.ObjectPutCondition{{}, {IfAbsent: true, IfVersion: "both"}} {
		if _, err := backend.Put(t.Context(), key, nil, condition); !errors.IsValidationError(err) {
			t.Fatalf("invalid condition: %v", err)
		}
	}
	if _, err := backend.Put(t.Context(), key, make([]byte, 1025), storage.ObjectPutCondition{IfAbsent: true}); !errors.IsValidationError(err) {
		t.Fatalf("oversized write: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := backend.Put(ctx, key, nil, storage.ObjectPutCondition{IfAbsent: true}); err == nil {
		t.Fatal("canceled writer created a record")
	}
	if _, err := backend.Get(t.Context(), key); !errors.IsNotFound(err) {
		t.Fatalf("refused operations changed storage: %v", err)
	}
}

func TestCoordinationBackendRefusesReplicaReadsAndWrites(t *testing.T) {
	if os.Getenv("STARMAP_TEST_COORDINATION_SERVER_CHANGES") != "1" {
		t.Skip("replica qualification requires an explicitly isolated test service")
	}
	backend, connection := testBackend(t)
	key := strconv.FormatInt(time.Now().UnixNano(), 10)
	first, err := backend.Put(t.Context(), key, []byte("primary"), storage.ObjectPutCondition{IfAbsent: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Do(t.Context(), connection.B().Arbitrary("REPLICAOF").Args("127.0.0.1", "1").Build()).Error(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := connection.Do(ctx, connection.B().Arbitrary("REPLICAOF").Args("NO", "ONE").Build()).Error(); err != nil {
			t.Error(err)
		}
	})
	if _, err := backend.GetCurrent(t.Context(), key); err == nil {
		t.Fatal("replica supplied authoritative coordination state")
	}
	if _, err := backend.Put(t.Context(), key, []byte("replica"), storage.ObjectPutCondition{IfVersion: first.Version}); err == nil {
		t.Fatal("replica accepted a coordination write")
	}
}

func TestCoordinationBackendConfigurationIsInert(t *testing.T) {
	_, connection := testBackend(t)
	connection.Close()
	if _, err := New(connection, Config{Prefix: "closed-client"}); err != nil {
		t.Fatalf("construction accessed a closed client: %v", err)
	}
	if _, err := New(nil, Config{Prefix: "missing-client"}); !errors.IsValidationError(err) {
		t.Fatalf("missing client: %v", err)
	}
	for _, config := range []Config{{}, {Prefix: "invalid\n"}, {Prefix: "negative", MaxObjectBytes: -1}, {Prefix: "unbounded", MaxObjectBytes: MaxObjectBytes + 1}} {
		if _, err := New(connection, config); !errors.IsValidationError(err) {
			t.Fatalf("invalid configuration: %+v, %v", config, err)
		}
	}
}

func TestCoordinationBackendRetainsStateAcrossServerRestart(t *testing.T) {
	container := os.Getenv("STARMAP_TEST_COORDINATION_CONTAINER")
	if os.Getenv("STARMAP_TEST_COORDINATION_SERVER_CHANGES") != "1" || container == "" {
		t.Skip("restart qualification requires an explicitly isolated test container")
	}
	backend, connection := testBackend(t)
	key := strconv.FormatInt(time.Now().UnixNano(), 10)
	first, err := backend.Put(t.Context(), key, []byte("retained-through-restart"), storage.ObjectPutCondition{IfAbsent: true})
	if err != nil {
		t.Fatal(err)
	}
	connection.Close()
	if output, err := exec.CommandContext(t.Context(), "docker", "restart", container).CombinedOutput(); err != nil {
		t.Fatalf("restart isolated coordination service: %v: %s", err, output)
	}
	waitForRestartedService(t)
	reopened, _ := testBackend(t)
	current, err := reopened.GetCurrent(t.Context(), key)
	if err != nil || current.Version != first.Version || !bytes.Equal(current.Data, first.Data) {
		t.Fatalf("restart lost the accepted conditional record: %+v, %v", current, err)
	}
	if _, err := reopened.Put(t.Context(), key, []byte("resumed"), storage.ObjectPutCondition{IfVersion: first.Version}); err != nil {
		t.Fatalf("restart lost the conditional version: %v", err)
	}
}

func waitForRestartedService(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for {
		connection, err := client.NewClient(client.ClientOption{
			InitAddress:  []string{os.Getenv("STARMAP_TEST_VALKEY_ADDRESS")},
			DisableCache: true, DisableRetry: true, Dialer: net.Dialer{Timeout: time.Second},
		})
		if err == nil {
			connection.Close()
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("restarted coordination service did not become ready: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
	}
}
