package app

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
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

type debugEncryptMessage struct {
	Ciphertext []byte `json:"ciphertext"`
	HMAC       []byte `json:"hmac"`
}

func (a *App) debugEncrypt(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	p, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ciphertext, err := simplecrypto.Encrypt(a.aesKey, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := debugEncryptMessage{
		Ciphertext: ciphertext,
		HMAC:       simplecrypto.HMAC(a.aesKey, ciphertext),
	}

	render.Respond(w, r, resp)
}

func (a *App) debugDecrypt(w http.ResponseWriter, r *http.Request) {
	var m debugEncryptMessage

	if err := render.Decode(r, &m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !simplecrypto.CheckMAC(a.aesKey, m.Ciphertext, m.HMAC) {
		http.Error(w, "provided hmac did not match ciphertext", http.StatusBadRequest)
		return
	}

	payload, err := simplecrypto.Decrypt(a.aesKey, m.Ciphertext)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(payload)
}
