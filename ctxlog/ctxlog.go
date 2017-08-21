package ctxlog

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

type loggerKey struct{}

var nopLogger = zap.NewNop()

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
