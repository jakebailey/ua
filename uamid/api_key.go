package uamid

import (
	"context"
	"net/http"

	"github.com/jakebailey/ua/ctxlog"
	"go.uber.org/zap"
)

const apiKeyHeader = "X-Api-Key"

type apiNameKey struct{}

// APIKey protects a handler with API keys. apiKeyNames is a map from API
// key strings to human-readable names. The name is added to the zap logger.
//
// API keys are passed through the X-Api-Key header. If the header is not
// blank and exists in the map, then the name associated with the API key is
// added to the request context and logger.
func APIKey(apiKeyNames map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			key := r.Header.Get(apiKeyHeader)

			name, ok := apiKeyNames[key]
			if key != "" && ok {
				ctx = context.WithValue(ctx, apiNameKey{}, name)
				ctx, _ = ctxlog.FromContextWith(ctx,
					zap.String("api_user", name),
				)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// APIKeyProtect checks the request for an API key name set by the APIKey handler
// and writes StatusUnauthorized to the response if none is set.
func APIKeyProtect(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if name, _ := r.Context().Value(apiNameKey{}).(string); name == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
