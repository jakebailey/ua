package ctxlog

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

type loggerKey struct{}

var nopLogger = zap.NewNop()

// Logger adds a logger to a Handler chain.
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := WithLogger(r.Context(), logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

// FromContext gets a zap logger from a context. If none is set, then a nop
// logger is returned.
func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok {
		return logger
	}
	return nopLogger
}

// FromRequest gets a zap logger from a request's context. If none is set,
// then a nop logger is returned.
func FromRequest(r *http.Request) *zap.Logger {
	return FromContext(r.Context())
}

// WithLogger adds a zap logger to a context.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContextWith gets a logger from a context, adds the specified fields,
// and returns context with the new logger.
func FromContextWith(ctx context.Context, fields ...zap.Field) (context.Context, *zap.Logger) {
	if len(fields) == 0 {
		return ctx, FromContext(ctx)
	}

	logger := FromContext(ctx).With(fields...)
	return WithLogger(ctx, logger), logger
}
