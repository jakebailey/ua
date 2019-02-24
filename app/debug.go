package app

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/e-dard/netbug"
	"github.com/go-chi/chi"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/simplecrypto"
	"go.uber.org/zap"
)

func (a *App) routeDebug(r chi.Router) {
	a.routeDebugProd(r)

	r.Group(func(r chi.Router) {
		r.Use(a.precheckDockerMiddleware, a.precheckDatabaseMiddleware)

		r.Route("/trigger", func(r chi.Router) {
			r.Get("/clean_inactive", a.triggerCleanInactive)
			r.Get("/check_expired", a.triggerCheckExpired)
		})

		r.Route("/crypto", func(r chi.Router) {
			r.Post("/encrypt", a.debugEncrypt)
			r.Post("/decrypt", a.debugDecrypt)
		})
	})
}

func (a *App) routeDebugProd(r chi.Router) {
	var pprofHandler http.Handler
	if a.config.Debug {
		pprofHandler = netbug.Handler()
	} else if a.config.PProfToken != "" {
		pprofHandler = netbug.AuthHandler(a.config.PProfToken)
	}

	if pprofHandler != nil {
		// This removes the need for this function to know anything about where it is being routed,
		// i.e. no hardcoding of "/debug".
		fixer := func(w http.ResponseWriter, r *http.Request) {
			ctx := chi.RouteContext(r.Context())

			prefix := ""

			for _, pat := range ctx.RoutePatterns {
				prefix += strings.TrimSuffix(pat, "/*")
			}

			prefix += "/"

			http.StripPrefix(prefix, pprofHandler).ServeHTTP(w, r)
		}

		// pprofHandler = http.StripPrefix("/debug/pprof/", pprofHandler)
		r.Handle("/pprof/*", http.HandlerFunc(fixer))
	}

	r.Get("/trigger/checks", func(w http.ResponseWriter, r *http.Request) {
		logger := ctxlog.FromRequest(r)

		// TODO: Change pprofToken into a generic debug password.
		if !a.config.Debug && r.FormValue("token") != a.config.PProfToken {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		a.dockerCheckRunner.Run()
		a.databaseCheckRunner.Run()

		if _, err := w.Write([]byte("ok")); err != nil {
			logger.Error("error writing response",
				zap.Error(err),
			)
		}
	})
}

func (a *App) triggerCleanInactive(w http.ResponseWriter, r *http.Request) {
	logger := ctxlog.FromRequest(r)

	a.cleanInactiveRunner.Run()

	if _, err := w.Write([]byte("ok")); err != nil {
		logger.Error("error writing response",
			zap.Error(err),
		)
	}
}

func (a *App) triggerCheckExpired(w http.ResponseWriter, r *http.Request) {
	logger := ctxlog.FromRequest(r)

	a.checkExpiredRunner.Run()

	if _, err := w.Write([]byte("ok")); err != nil {
		logger.Error("error writing response",
			zap.Error(err),
		)
	}
}

func (a *App) debugEncrypt(w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := simplecrypto.EncodeJSONWriter(a.aesKey, payload, w); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

}

func (a *App) debugDecrypt(w http.ResponseWriter, r *http.Request) {
	logger := ctxlog.FromRequest(r)

	payload, err := simplecrypto.DecodeJSONReader(a.aesKey, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := w.Write(payload); err != nil {
		logger.Error("error writing response",
			zap.Error(err),
		)
	}
}
