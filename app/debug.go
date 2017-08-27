package app

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/jakebailey/ua/simplecrypto"
)

func (a *App) routeDebug(r chi.Router) {
	r.Route("/trigger", func(r chi.Router) {
		r.Get("/clean_inactive", a.triggerCleanInactive)
		r.Get("/check_expired", a.triggerCheckExpired)
	})

	r.Route("/crypto", func(r chi.Router) {
		r.Post("/encrypt", a.debugEncrypt)
		r.Post("/decrypt", a.debugDecrypt)
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

func (a *App) debugEncrypt(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := simplecrypto.EncodeJSON(a.aesKey, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(data)
}

func (a *App) debugDecrypt(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	payload, err := simplecrypto.DecodeJSON(a.aesKey, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(payload)
}
