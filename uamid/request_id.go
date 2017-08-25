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

type requestIDKey struct{}

const requestIDHeader = "X-Request-ID"

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

// RequestID manages request IDs, similarly to Heroku's method.
// If the client sets X-Request-ID, then it is used. If no request ID
// is provided, or the provided request ID is not between 20 and 200
// characters (inclusive), then a ULID is generated and used.
//
// The request ID is also written to the ResponseWriter, and if a zap
// logger is in the request context, added with the provided field key.
//
// NOTE: The http.Handler interface requires that handlers not modify
// the request. In order to get the "real" request ID, use the
// GetRequestID function.
func RequestID(fieldKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			requestID := r.Header.Get(requestIDHeader)
			if len(requestID) < 20 || len(requestID) > 200 {
				requestID = newULID().String()
			}

			w.Header().Set(requestIDHeader, requestID)
			ctx = context.WithValue(ctx, requestIDKey{}, requestID)

			if fieldKey != "" {
				ctx, _ = ctxlog.FromContextWith(ctx,
					zap.String(fieldKey, requestID),
				)
			}

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// GetRequestID gets the request id from a request's context. If none was set
// (i.e. the RequestID middleware wasn't run), then an empty string is returned.
func GetRequestID(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDKey{}).(string)
	return requestID
}
