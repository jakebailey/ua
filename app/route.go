package app

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/rs/zerolog/hlog"
)

func (a *App) route() {
	r := chi.NewRouter()

	r.Use(hlog.NewHandler(a.log))
	r.Use(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Msg("http request")
	}))
	r.Use(hlog.RequestIDHandler("req_id", "Request-Id"))

	r.Use(func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					log := hlog.FromRequest(r)

					stack := string(debug.Stack())

					log.Error().
						Interface("panic_value", rvr).
						Str("stack", stack).
						Msg("PANIC")

					if a.debug {
						http.Error(w, stack, http.StatusInternalServerError)
					} else {
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
			}()

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	})

	r.Use(middleware.CloseNotify)
	r.Use(middleware.Heartbeat("/ping"))

	routeStatic(r, "/static", a.staticPath)

	r.Route("/assignments", a.routeAssignments)
	r.Route("/containers", a.routeContainers)
	r.Route("/specs", a.routeSpecs)

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
