package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAzureDefaultSecretCredentialChain(t *testing.T) {
	for _, mode := range []string{"environment", "workload-identity", "denied"} {
		t.Run(mode, func(t *testing.T) {
			clearSecretSDKEnvironment(t)
			t.Setenv("AZURE_TOKEN_CREDENTIALS", "prod")
			t.Setenv("AZURE_TENANT_ID", "fixture-tenant")
			t.Setenv("AZURE_CLIENT_ID", "fixture-client")
			t.Setenv("AZURE_AUTHORITY_HOST", "https://login.microsoftonline.com")
			if mode == "workload-identity" {
				t.Setenv("AZURE_FEDERATED_TOKEN_FILE", secretSDKFixtureFile(t, "assertion", "fixture-workload-assertion"))
			} else {
				t.Setenv("AZURE_CLIENT_SECRET", "fixture-client-secret")
			}
			var tokenCalls, authorizedReads atomic.Int32
			localSecretSDKTransport(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				authority := "https://login.microsoftonline.com/fixture-tenant"
				switch {
				case req.Host == "login.microsoftonline.com" && req.URL.Path == "/common/discovery/instance":
					if !strings.HasPrefix(req.URL.Query().Get("authorization_endpoint"), authority+"/") {
						t.Error("wrong discovery tenant")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"tenant_discovery_endpoint": authority + "/v2.0/.well-known/openid-configuration", "metadata": []any{map[string]any{"preferred_network": "login.microsoftonline.com", "preferred_cache": "login.microsoftonline.com", "aliases": []string{"login.microsoftonline.com"}}}})
				case req.Host == "login.microsoftonline.com" && req.URL.Path == "/fixture-tenant/v2.0/.well-known/openid-configuration":
					_ = json.NewEncoder(w).Encode(map[string]string{"token_endpoint": authority + "/oauth2/v2.0/token", "authorization_endpoint": authority + "/oauth2/v2.0/authorize", "issuer": authority + "/v2.0"})
				case req.Host == "login.microsoftonline.com" && req.URL.Path == "/fixture-tenant/oauth2/v2.0/token":
					tokenCalls.Add(1)
					if err := req.ParseForm(); err != nil {
						t.Error(err)
					}
					if req.Method != http.MethodPost || req.Form.Get("grant_type") != "client_credentials" || req.Form.Get("client_id") != "fixture-client" {
						t.Error("wrong token exchange")
					}
					if req.Form.Get("scope") != "https://vault.azure.net/.default openid offline_access profile" {
						t.Errorf("wrong token scope: %q", req.Form.Get("scope"))
					}
					if mode == "workload-identity" {
						if req.Form.Get("client_assertion") != "fixture-workload-assertion" || req.Form.Get("client_secret") != "" {
							t.Error("workload assertion was not selected")
						}
					} else if req.Form.Get("client_secret") != "fixture-client-secret" {
						t.Error("environment credential was not selected")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fixture-access-token", "token_type": "Bearer", "expires_in": 3600})
				case req.Host == "catalog.vault.azure.net" && req.URL.Path == "/secrets/provider-key/7":
					if req.Header.Get("Authorization") == "" {
						w.Header().Set("WWW-Authenticate", `Bearer authorization="https://login.microsoftonline.com/fixture-tenant" resource="https://vault.azure.net"`)
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					authorizedReads.Add(1)
					if req.Header.Get("Authorization") != "Bearer fixture-access-token" {
						t.Error("wrong bearer token")
					}
					if mode == "denied" {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"error":{"code":"Forbidden","message":"fixture denial"}}`))
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"value": `{"api-key":"fixture-provider-value"}`, "id": "https://catalog.vault.azure.net/secrets/provider-key/7"})
				default:
					t.Errorf("unexpected SDK route: %s %s", req.Host, req.URL.Path)
					http.Error(w, "unexpected fixture route", http.StatusBadRequest)
				}
			}), "login.microsoftonline.com", "catalog.vault.azure.net")
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			material, err := newAzureKeyVaultSource().Resolve(ctx, mustReference(t, "azure-key-vault:https://catalog.vault.azure.net/secrets/provider-key?version=7#api-key"))
			if mode == "denied" {
				if !isSourceError(err, SourceErrorDenied) || len(material.values) != 0 {
					t.Fatalf("denied secret: error=%v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if material.values["api-key"] != "fixture-provider-value" || material.version != "7" {
					t.Fatal("wrong secret material")
				}
			}
			if tokenCalls.Load() != 1 || authorizedReads.Load() != 1 {
				t.Fatalf("token exchanges=%d secret reads=%d, want 1 each", tokenCalls.Load(), authorizedReads.Load())
			}
		})
	}
}
