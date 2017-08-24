package app

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
)

func (a *App) routeDebug(r chi.Router) {
	r.Route("/trigger", func(r chi.Router) {
		r.Get("/clean_inactive", a.triggerCleanInactive)
		r.Get("/check_expired", a.triggerCheckExpired)
	})
}

func (a *App) triggerCleanInactive(w http.ResponseWriter, r *http.Request) {
	a.cleanInactiveRunner.Run()
	fmt.Fprint(w, "ok")
}

func (a *App) triggerCheckExpired(w http.ResponseWriter, r *http.Request) {
	a.checkExpiredRunner.Run()
	fmt.Fprint(w, "ok")
}
