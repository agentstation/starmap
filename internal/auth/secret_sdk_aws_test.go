package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAWSDefaultSecretCredentialChain(t *testing.T) {
	for _, mode := range []string{"environment", "shared-file", "environment-precedence", "web-identity", "denied", "missing"} {
		t.Run(mode, func(t *testing.T) {
			clearSecretSDKEnvironment(t)
			t.Setenv("AWS_REGION", "us-east-1")
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			t.Setenv("AWS_ENDPOINT_URL_SECRETS_MANAGER", "https://secrets.fixture.test")
			t.Setenv("AWS_CONFIG_FILE", secretSDKFixtureFile(t, "config", ""))
			t.Setenv("AWS_MAX_ATTEMPTS", "1")
			t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "false")
			shared := ""
			if mode == "shared-file" || mode == "environment-precedence" {
				shared = "[default]\naws_access_key_id = fixture-file-access\naws_secret_access_key = fixture-file-secret\n"
			}
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", secretSDKFixtureFile(t, "credentials", shared))
			expectedAccess := "fixture-env-access"
			expectedToken := "fixture-session-token"
			if mode == "web-identity" {
				expectedAccess = "fixture-role-access"
				expectedToken = "fixture-role-token"
				t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", secretSDKFixtureFile(t, "assertion", "fixture-web-identity-token"))
				t.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/fixture")
				t.Setenv("AWS_ROLE_SESSION_NAME", "fixture-session")
				t.Setenv("AWS_ENDPOINT_URL_STS", "https://sts.fixture.test")
			} else if mode == "shared-file" {
				expectedAccess = "fixture-file-access"
			} else if mode != "missing" {
				t.Setenv("AWS_ACCESS_KEY_ID", expectedAccess)
				t.Setenv("AWS_SECRET_ACCESS_KEY", "fixture-env-secret")
				t.Setenv("AWS_SESSION_TOKEN", "fixture-session-token")
			}
			var calls, roleCalls atomic.Int32
			version := "00000000000000000000000000000007"
			localSecretSDKTransport(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Host == "sts.fixture.test" {
					roleCalls.Add(1)
					if err := req.ParseForm(); err != nil {
						t.Error(err)
					}
					if req.Form.Get("Action") != "AssumeRoleWithWebIdentity" || req.Form.Get("WebIdentityToken") != "fixture-web-identity-token" || req.Form.Get("RoleArn") != "arn:aws:iam::123456789012:role/fixture" || req.Form.Get("RoleSessionName") != "fixture-session" {
						t.Error("wrong web identity exchange")
					}
					w.Header().Set("Content-Type", "text/xml")
					_, _ = fmt.Fprintf(w, `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleWithWebIdentityResult><Credentials><AccessKeyId>fixture-role-access</AccessKeyId><SecretAccessKey>fixture-role-secret</SecretAccessKey><SessionToken>fixture-role-token</SessionToken><Expiration>%s</Expiration></Credentials></AssumeRoleWithWebIdentityResult></AssumeRoleWithWebIdentityResponse>`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
					return
				}
				calls.Add(1)
				if req.Method != http.MethodPost || req.Header.Get("X-Amz-Target") != "secretsmanager.GetSecretValue" {
					t.Error("unexpected secret operation")
				}
				if !strings.HasPrefix(req.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") || !strings.Contains(req.Header.Get("Authorization"), "/us-east-1/secretsmanager/aws4_request") {
					t.Error("wrong signature algorithm or service scope")
				}
				if !strings.Contains(req.Header.Get("Authorization"), "Credential="+expectedAccess+"/") {
					t.Error("request uses the wrong credential source")
				}
				if mode != "shared-file" && req.Header.Get("X-Amz-Security-Token") != expectedToken {
					t.Error("session token is absent")
				}
				var input struct {
					SecretID  string `json:"SecretId"`
					VersionID string `json:"VersionId"`
				}
				if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				if input.SecretID != "provider/key" || input.VersionID != version {
					t.Error("request lost secret identity or version")
				}
				w.Header().Set("Content-Type", "application/x-amz-json-1.1")
				if mode == "denied" {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"__type":"AccessDeniedException","message":"fixture denial"}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"VersionId": version, "SecretString": `{"api-key":"fixture-provider-value"}`})
			}), "secrets.fixture.test", "sts.fixture.test")
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			material, err := newAWSSecretsManagerSource().Resolve(ctx, mustReference(t, "aws-secrets-manager:provider/key?version="+version+"#api-key"))
			switch mode {
			case "missing":
				if !isSourceError(err, SourceErrorUnavailable) || len(material.values) != 0 || calls.Load() != 0 {
					t.Fatalf("missing credentials: error=%v calls=%d", err, calls.Load())
				}
			case "denied":
				if !isSourceError(err, SourceErrorDenied) || len(material.values) != 0 {
					t.Fatalf("denied secret: error=%v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if material.values["api-key"] != "fixture-provider-value" || material.version != version {
					t.Fatal("wrong secret material")
				}
			}
			expectedRoleCalls := int32(0)
			if mode == "web-identity" {
				expectedRoleCalls = 1
			}
			if roleCalls.Load() != expectedRoleCalls {
				t.Fatalf("role exchanges=%d, want %d", roleCalls.Load(), expectedRoleCalls)
			}
			if mode != "missing" && calls.Load() != 1 {
				t.Fatalf("secret requests=%d, want 1", calls.Load())
			}
		})
	}
}
