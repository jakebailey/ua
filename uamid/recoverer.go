package uamid

import (
	"net/http"

	"github.com/jakebailey/ua/ctxlog"
	"go.uber.org/zap"
)

// Recoverer recovers from panics, and reports an internal server error.
// The panic will be logged, along with a stack trace.
func Recoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				logger := ctxlog.FromRequest(r)

				// Ensure logger is logging stack traces, at least here.
				logger = logger.WithOptions(zap.AddStacktrace(zap.ErrorLevel))

				logger.Error("PANIC",
					zap.Any("panic_value", rvr),
				)

				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
