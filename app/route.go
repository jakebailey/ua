package app

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/static"
	"github.com/jakebailey/ua/uamid"
)

func (a *App) route() {
	r := chi.NewRouter()

	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Connection", "Upgrade"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(cors.Handler)

	r.Use(ctxlog.Logger(a.logger))
	r.Use(uamid.RequestID("request_id"))
	r.Use(uamid.RequestLogger)
	r.Use(uamid.Recoverer)

	r.Route("/health", a.routeHealth)

	if a.staticPath == "" {
		r.Handle("/static/*", http.StripPrefix("/static", http.FileServer(static.FS(false))))
	} else {
		routeStatic(r, "/static", a.staticPath)
	}

	r.Handle("/favicon.ico", http.RedirectHandler("/static/favicon.ico", http.StatusFound))

	r.Route("/spec", a.routeSpec)
	r.Route("/instance", a.routeInstance)

	if a.debug {
		r.Route("/debug", a.routeDebug)
	} else {
		r.Route("/debug", a.routeDebugProd)
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
