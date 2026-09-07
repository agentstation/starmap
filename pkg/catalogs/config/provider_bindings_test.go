package config_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

const bindingsSetting = "STARMAP_CATALOG_PROVIDER_BINDINGS"

func configuredBinding(id string) sources.ProviderAcquisitionBinding {
	return sources.ProviderAcquisitionBinding{
		SchemaVersion: 1, ID: id, Revision: "1", ProviderID: "openai", AccountID: id,
		Region: "global", APISurface: "models.list", CredentialRole: sources.ProviderBindingCatalogAcquisition,
		CredentialProfileID: "api-key",
	}
}

func bindingJSON(t *testing.T, bindings ...sources.ProviderAcquisitionBinding) string {
	t.Helper()
	data, err := json.Marshal(bindings)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestProviderBindingSettingsRejectInvalidDeclarations(t *testing.T) {
	valid := bindingJSON(t, configuredBinding("one"))
	for name, value := range map[string]string{
		"empty": "", "null": "null", "object": "{}", "null-entry": "[null]", "scalar-entry": "[1]",
		"trailing": valid + " []", "missing-field": "[{}]",
		"unknown-field": strings.Replace(valid, `"id":`, `"api_key":"private-fixture","id":`, 1),
		"duplicate-id":  bindingJSON(t, configuredBinding("one"), configuredBinding("one")),
		"invalid-role":  strings.Replace(valid, "catalog_acquisition", "inference", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := config.Parse(map[string]string{bindingsSetting: value})
			if err == nil {
				t.Fatal("accepted invalid provider binding declarations")
			}
			if strings.Contains(err.Error(), "private-fixture") {
				t.Fatal("configuration error exposed a supplied credential value")
			}
		})
	}
}

func TestProviderBindingSettingsReachRuntimeWithoutAmbientFallback(t *testing.T) {
	bindings := []sources.ProviderAcquisitionBinding{configuredBinding("one"), configuredBinding("two")}
	for _, test := range []struct {
		name   string
		layers []config.Layer
		want   []sources.ProviderAcquisitionBinding
		legacy bool
	}{
		{name: "omitted", legacy: true},
		{name: "empty", layers: []config.Layer{{Name: "file", Values: map[string]string{bindingsSetting: "[]"}}}},
		{name: "two-scopes", layers: []config.Layer{{Name: "file", Values: map[string]string{bindingsSetting: bindingJSON(t, bindings...)}}}, want: bindings},
		{name: "empty-replaces-file", layers: []config.Layer{
			{Name: "environment", Values: map[string]string{bindingsSetting: "[]"}},
			{Name: "file", Values: map[string]string{bindingsSetting: bindingJSON(t, bindings...)}},
		}},
		{name: "array-replaces-file", layers: []config.Layer{
			{Name: "environment", Values: map[string]string{bindingsSetting: bindingJSON(t, bindings[1])}},
			{Name: "file", Values: map[string]string{bindingsSetting: bindingJSON(t, bindings...)}},
		}, want: bindings[1:]},
		{name: "source-replacement-keeps-bindings", layers: []config.Layer{
			{Name: "environment", Values: map[string]string{config.Source: "embedded"}},
			{Name: "file", Values: map[string]string{config.Source: "public", bindingsSetting: bindingJSON(t, bindings...)}},
		}, want: bindings},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := config.Resolve(test.layers...)
			if err != nil {
				t.Fatal(err)
			}
			observer := &bindingSettingsAcquirer{}
			options := []runtime.Option{runtime.WithCatalogSource("embedded"), runtime.WithAcquisitionEnabled(false),
				runtime.WithSourcePollInterval(0), runtime.WithStateDirectory(filepath.Join(t.TempDir(), "runtime")),
				runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory())), runtime.WithAcquirer(observer)}
			options = append(options, resolved.Config.Options()...)
			connected, err := runtime.Open(t.Context(), options...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := connected.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := connected.Sync(t.Context()); err != nil {
				t.Fatal(err)
			}
			if observer.legacy != test.legacy || !reflect.DeepEqual(observer.bindings, test.want) {
				t.Fatalf("acquisition selection = legacy %t, bindings %#v; want legacy %t, bindings %#v", observer.legacy, observer.bindings, test.legacy, test.want)
			}
		})
	}
}

type bindingSettingsAcquirer struct {
	legacy   bool
	bindings []sources.ProviderAcquisitionBinding
}

func (a *bindingSettingsAcquirer) AcquireProviders(context.Context, runtime.AcquisitionRequest) (runtime.AcquisitionResult, error) {
	a.legacy = true
	return runtime.AcquisitionResult{}, nil
}

func (a *bindingSettingsAcquirer) AcquireProviderBindings(_ context.Context, _ runtime.AcquisitionRequest, bindings []sources.ProviderAcquisitionBinding) (runtime.AcquisitionResult, error) {
	a.bindings = bindings
	return runtime.AcquisitionResult{}, nil
}
