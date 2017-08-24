package app

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jakebailey/ua/uamid"
)

func (a *App) route() {
	r := chi.NewRouter()

	r.Use(uamid.Logger(a.logger))
	r.Use(uamid.RequestID("request_id"))
	r.Use(uamid.RequestLogger)
	r.Use(uamid.Recoverer)
	r.Use(middleware.CloseNotify)
	r.Use(middleware.Heartbeat("/ping"))

	routeStatic(r, "/static", a.staticPath)
	r.Handle("/favicon.ico", http.RedirectHandler("/static/favicon.ico", http.StatusFound))

	r.Route("/specs", a.routeSpecs)
	r.Route("/term", a.routeTerm)

	if a.debug {
		r.Route("/debug", a.routeDebug)
	}

	a.router = r
}

func routeStatic(r chi.Router, httpPath string, fsPath string) {
	root := http.Dir(fsPath)
	fs := http.StripPrefix(httpPath, http.FileServer(root))

	r.Group(func(r chi.Router) {
		r.Use(middleware.DefaultCompress)

		if httpPath != "/" && httpPath[len(httpPath)-1] != '/' {
			r.Handle(httpPath, http.RedirectHandler(httpPath+"/", http.StatusFound))
			httpPath += "/"
		}
		httpPath += "*"

		r.Handle(httpPath, fs)
	})
}
