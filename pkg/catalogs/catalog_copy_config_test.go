package catalogs

import (
	stderrors "errors"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
)

func TestCatalogCopyOwnsSaveConfiguration(t *testing.T) {
	original := NewEmpty()
	copied, err := original.Copy()
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if err := copied.Save(save.WithPath(t.TempDir())); err != nil {
		t.Fatalf("Save copy: %v", err)
	}

	err = original.Save()
	if err == nil {
		t.Fatal("Saving a copy configured the original catalog write path")
	}
	var configErr *errors.ConfigError
	if !stderrors.As(err, &configErr) {
		t.Fatalf("Original save error = %T, want *errors.ConfigError", err)
	}
}

func TestCatalogCopyOwnsMergeStrategy(t *testing.T) {
	original := NewEmpty()
	copied, err := original.Copy()
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	copied.SetMergeStrategy(MergeReplaceAll)
	if got := original.MergeStrategy(); got != MergeEnrichEmpty {
		t.Fatalf("Original merge strategy = %v, want %v", got, MergeEnrichEmpty)
	}
}

func TestCatalogCopyConfigurationIsRaceIndependent(t *testing.T) {
	original := NewEmpty()
	copied, err := original.Copy()
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 1_000 {
			copied.SetMergeStrategy(MergeReplaceAll)
			copied.SetMergeStrategy(MergeAppendOnly)
		}
	}()
	go func() {
		defer wg.Done()
		for range 1_000 {
			_ = original.MergeStrategy()
		}
	}()
	wg.Wait()
}
