package catalogstore

import (
	"context"
	"database/sql"
	stderrors "errors"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"

	_ "modernc.org/sqlite"
)

func TestSQLCatalogStoreAtomicRollbackReadCurrent(t *testing.T) {
	store, ok := newSQLConformanceStore(t).(*SQL)
	if !ok {
		t.Fatal("SQL factory returned unexpected store type")
	}
	first := testGeneration("sql-rollback-first", "first")
	if err := store.Commit(context.Background(), first, ""); err != nil {
		t.Fatalf("Commit first: %v", err)
	}

	fault := stderrors.New("injected before SQL commit")
	store.beforeCommit = func() error { return fault }
	second := testGeneration("sql-rollback-second", "second")
	if err := store.Commit(context.Background(), second, "sql-rollback-first"); !stderrors.Is(err, fault) {
		t.Fatalf("Commit second error = %v, want injected fault", err)
	}
	assertStoredGeneration(t, store, first)
	if _, err := store.Get(context.Background(), "sql-rollback-second"); !pkgerrors.IsNotFound(err) {
		t.Fatalf("rolled-back candidate error = %v, want not found", err)
	}

	store.beforeCommit = nil
	if err := store.Commit(context.Background(), second, "sql-rollback-first"); err != nil {
		t.Fatalf("retry second: %v", err)
	}
	assertStoredGeneration(t, store, second)
	if err := store.Commit(context.Background(), first, "sql-rollback-second"); err != nil {
		t.Fatalf("reactivate retained first generation: %v", err)
	}
	assertStoredGeneration(t, store, first)
}

func newSQLConformanceStore(t *testing.T) Store {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatalf("Open SQLite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("Ping SQLite: %v", err)
	}
	store, err := NewSQL(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQL: %v", err)
	}
	return store
}

func TestSQLCatalogStoreReopensCurrentGeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("Open first SQLite: %v", err)
	}
	db.SetMaxOpenConns(1)
	first, err := NewSQL(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQL first: %v", err)
	}
	want := testGeneration("sql-reopen", "durable")
	if err := first.Commit(context.Background(), want, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close first SQLite: %v", err)
	}

	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("Open second SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	reopened, err := NewSQL(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQL reopened: %v", err)
	}
	got, err := reopened.Current(context.Background())
	if err != nil {
		t.Fatalf("Current reopened: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("reopened generation mismatch (-want +got):\n%s", diff)
	}
}
