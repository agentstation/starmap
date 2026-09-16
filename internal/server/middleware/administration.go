package middleware

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/agentstation/starmap/server/administration"
)

type principalContextKey struct{}

// AdministrativePrincipal returns the credential identity authenticated for this request.
func AdministrativePrincipal(ctx context.Context) (administration.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(administration.Principal)
	return principal, ok
}

// RequiresAdministrator classifies server diagnostics and registered mutation routes before route dispatch.
func RequiresAdministrator(request *http.Request, prefix string) bool {
	name := path.Clean(request.URL.Path)
	return name == "/admin" || strings.HasPrefix(name, "/admin/") || name == "/metrics" ||
		name == prefix+"/stats" || name == prefix+"/update" || name == prefix+"/updates" ||
		strings.HasPrefix(name, prefix+"/updates/") && name != prefix+"/updates/stream"
}

// AdministrationAccess requires separate administrator authorization before administrative route dispatch.
// With no administration manager, all administrative routes remain disabled.
func AdministrationAccess(manager *administration.Manager, audience, header, prefix string, publicPaths []string, unauthorized func(http.ResponseWriter, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			administrative := RequiresAdministrator(request, prefix)
			if manager == nil {
				if administrative {
					http.Error(w, "Administrator access is unavailable", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, request)
				return
			}
			if !administrative {
				for _, public := range publicPaths {
					if request.URL.Path == public {
						next.ServeHTTP(w, request)
						return
					}
				}
			}
			token := request.Header.Get(header)
			if token == "" {
				value := request.Header.Get("Authorization")
				if scheme, credential, found := strings.Cut(value, " "); found && strings.EqualFold(scheme, "Bearer") {
					token = credential
				}
			}
			principal, accepted := manager.Authenticate(token, audience)
			if !accepted {
				unauthorized(w, request)
				return
			}
			if administrative && principal.Role() != administration.Administrator {
				http.Error(w, "Administrator permission required", http.StatusForbidden)
				return
			}
			ctx := context.WithValue(request.Context(), principalContextKey{}, principal)
			if path.Clean(request.URL.Path) == prefix+"/updates/stream" {
				tracked, cancel := manager.Track(ctx, principal)
				defer cancel()
				ctx = tracked
			}

			next.ServeHTTP(w, request.WithContext(ctx))
		})
	}
}
