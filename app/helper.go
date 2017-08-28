package app

import (
	"net/http"
	"time"
)

// httpError writes a message to the writer. If the app is in debug mode,
// then the message will be written, otherwise the default message
// for the provided status code will be used.
func (a *App) httpError(w http.ResponseWriter, msg string, code int) {
	if a.debug {
		http.Error(w, msg, code)
	} else {
		http.Error(w, http.StatusText(code), code)
	}
}

func (a *App) instanceExpireTime() *time.Time {
	t := time.Now().Add(a.instanceExpire)
	return &t
}
