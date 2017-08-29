package app

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/jakebailey/ua/ctxlog"
	"go.uber.org/zap"
)

var healthOk = []byte("ok")

func (a *App) routeHealth(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(healthOk)
	})

	r.Get("/docker", a.healthDocker)
	r.Get("/database", a.healthDatabase)
}

func (a *App) healthDocker(w http.ResponseWriter, r *http.Request) {
	if _, err := a.cli.Info(r.Context()); err != nil {
		logger := ctxlog.FromRequest(r)
		logger.Error("error health checking docker",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(healthOk)
}

func (a *App) healthDatabase(w http.ResponseWriter, r *http.Request) {
	if err := a.db.Ping(); err != nil {
		logger := ctxlog.FromRequest(r)
		logger.Error("error health checking database",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(healthOk)
}
