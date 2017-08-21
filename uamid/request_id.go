package uamid

import (
	"context"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/jakebailey/ua/ctxlog"
	"github.com/oklog/ulid"
	"go.uber.org/zap"
)

type idKey struct{}

var randPool = &sync.Pool{
	New: func() interface{} {
		return rand.NewSource(time.Now().UnixNano())
	},
}

func newULID() ulid.ULID {
	entropy := randPool.Get().(rand.Source)
	id := ulid.MustNew(ulid.Now(), rand.New(entropy))
	randPool.Put(entropy)

	return id
}

// RequestID adds a request ID to the request context in the form of a ULID.
// If fieldKey is set, then the field will be added to the zap logger in the
// context. If headerName is set, then a HTTP header with that name will
// be set to the request ID.
func RequestID(fieldKey, headerName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			id, ok := ctx.Value(idKey{}).(ulid.ULID)
			if !ok {
				id = newULID()
				ctx = context.WithValue(ctx, idKey{}, id)
				r = r.WithContext(ctx)
			}
			if fieldKey != "" {
				logger := ctxlog.FromContext(ctx).With(zap.String(fieldKey, id.String()))
				r = r.WithContext(ctxlog.WithLogger(ctx, logger))
			}
			if headerName != "" {
				w.Header().Set(headerName, id.String())
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
