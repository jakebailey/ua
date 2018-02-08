package app

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/e-dard/netbug"
	"github.com/go-chi/chi"
	"github.com/jakebailey/ua/pkg/simplecrypto"
)

func (a *App) routeDebug(r chi.Router) {
	a.routeDebugProd(r)

	r.Route("/trigger", func(r chi.Router) {
		r.Get("/clean_inactive", a.triggerCleanInactive)
		r.Get("/check_expired", a.triggerCheckExpired)
	})

	r.Route("/crypto", func(r chi.Router) {
		r.Post("/encrypt", a.debugEncrypt)
		r.Post("/decrypt", a.debugDecrypt)
	})
}

func (a *App) routeDebugProd(r chi.Router) {
	var pprofHandler http.Handler
	if a.debug {
		pprofHandler = netbug.Handler()
	} else if a.pprofToken != "" {
		pprofHandler = netbug.AuthHandler(a.pprofToken)
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
}

func (a *App) triggerCleanInactive(w http.ResponseWriter, r *http.Request) {
	a.cleanInactiveRunner.Run()
	fmt.Fprint(w, "ok")
}

func (a *App) triggerCheckExpired(w http.ResponseWriter, r *http.Request) {
	a.checkExpiredRunner.Run()
	fmt.Fprint(w, "ok")
}

func (a *App) debugEncrypt(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

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
	defer r.Body.Close()

	payload, err := simplecrypto.DecodeJSONReader(a.aesKey, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(payload)
}
