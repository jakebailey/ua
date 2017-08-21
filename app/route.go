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
	r.Use(uamid.RequestID("request_id", "Request-Id"))
	r.Use(uamid.RequestLogger)
	r.Use(uamid.Recoverer)
	r.Use(middleware.CloseNotify)
	r.Use(middleware.Heartbeat("/ping"))

	routeStatic(r, "/static", a.staticPath)

	r.Route("/assignments", a.routeAssignments)
	r.Route("/containers", a.routeContainers)

	r.Route("/specs", a.routeSpecs)
	r.Route("/term", a.routeTerm)

	a.router = r
}

func routeStatic(r chi.Router, httpPath string, fsPath string) {
	root := http.Dir(fsPath)
	fs := http.StripPrefix(httpPath, http.FileServer(root))

	r.Group(func(r chi.Router) {
		r.Use(middleware.DefaultCompress)

		if httpPath != "/" && httpPath[len(httpPath)-1] != '/' {
			r.Get(httpPath, http.RedirectHandler(httpPath+"/", http.StatusFound).ServeHTTP)
			httpPath += "/"
		}
		httpPath += "*"

		r.Get(httpPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fs.ServeHTTP(w, r)
		}))
	})
}
