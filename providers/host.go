package providers

import (
	"fmt"
	"kiro-go/config"
	"net/http"
)

// Host is the surface the proxy core exposes to provider admin endpoints.
// Kept intentionally small: providers create accounts through the config
// package and only need the host to refresh its runtime caches.
type Host interface {
	// ReloadPool reloads the account pool after accounts were added or changed.
	ReloadPool()
	// RefreshAccountModels fetches and caches the model list for an account
	// (used for model-based routing).
	RefreshAccountModels(account *config.Account) error
}

// RouteHandler is an admin HTTP endpoint contributed by a provider package.
type RouteHandler func(Host, http.ResponseWriter, *http.Request)

// adminRoutes maps "METHOD path" to its handler. Each provider package
// registers its own auth/import endpoints from init(), so routes live next to
// the provider they belong to and the proxy core never changes when a
// provider is added.
var adminRoutes = map[string]RouteHandler{}

// RegisterAdminRoutes adds a provider's routes. Duplicate registration is a
// programming error caught at startup, not a silent overwrite at request time.
func RegisterAdminRoutes(routes map[string]RouteHandler) {
	for key, fn := range routes {
		if _, dup := adminRoutes[key]; dup {
			panic(fmt.Sprintf("admin route registered twice: %s", key))
		}
		adminRoutes[key] = fn
	}
}

// AdminRoute looks up the handler for an admin request.
func AdminRoute(method, path string) (RouteHandler, bool) {
	fn, ok := adminRoutes[method+" "+path]
	return fn, ok
}
