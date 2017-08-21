package uamid

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/jakebailey/ua/ctxlog"
	"go.uber.org/zap"
)

func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := ctxlog.WithLogger(r.Context(), logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

// RequestLogger logs information about HTTP requests using the zap logger
// in the request context.
func RequestLogger(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		defer func() {
			duration := time.Since(start)
			logger := ctxlog.FromRequest(r)

			logger.Info("http request",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.Int("status", ww.Status()),
				zap.Int("size", ww.BytesWritten()),
				zap.Duration("duration", duration),
			)
		}()

		next.ServeHTTP(ww, r)
	}
	return http.HandlerFunc(fn)
}
