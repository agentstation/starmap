package runtime

import (
	stderrors "errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestOriginPublicationRefusesImplicitAuthorityRemoval(t *testing.T) {
	for _, freshRuntimeState := range []bool{false, true} {
		name := "retained runtime state"
		if freshRuntimeState {
			name = "new runtime state"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "catalog-store")
			store, err := storage.NewFilesystem(path)
			if err != nil {
				t.Fatal(err)
			}
			directory := privateRuntimeDirectory(t)
			base := []Option{WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithSourcePollInterval(0)}
			origin := openTestRuntime(t, append(base, WithAuthorityOrigin(store, originTestConfig()))...)
			accepted, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if err := origin.Close(); err != nil {
				t.Fatal(err)
			}
			store, err = storage.NewFilesystem(path)
			if err != nil {
				t.Fatal(err)
			}
			if freshRuntimeState {
				base = append(base, WithStateDirectory(privateRuntimeDirectory(t)))
			}
			ordinary, openErr := Open(t.Context(), append(base, WithClientOptions(starmap.WithCatalogStore(store)))...)
			if ordinary != nil {
				if err := ordinary.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if openErr == nil {
				t.Error("removing origin configuration enabled an ordinary runtime")
			} else {
				var configErr *errors.ConfigError
				if !stderrors.As(openErr, &configErr) || configErr.Component != "catalog authority" {
					t.Errorf("authority selection error = %v", openErr)
				}
			}
			current, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(current, accepted) {
				t.Error("implicit authority removal changed the durable catalog")
			}
		})
	}
}
