package app

import (
	"encoding/base64"
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
	Ciphertext string `json:"ciphertext"`
	HMAC       string `json:"hmac"`
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

	hmac := simplecrypto.HMAC(a.aesKey, ciphertext)

	resp := debugEncryptMessage{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		HMAC:       base64.StdEncoding.EncodeToString(hmac),
	}

	render.Respond(w, r, resp)
}

func (a *App) debugDecrypt(w http.ResponseWriter, r *http.Request) {
	var m debugEncryptMessage

	if err := render.Decode(r, &m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ciphertext, err := base64.StdEncoding.DecodeString(m.Ciphertext)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hmac, err := base64.StdEncoding.DecodeString(m.HMAC)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !simplecrypto.CheckMAC(a.aesKey, ciphertext, hmac) {
		http.Error(w, "provided hmac did not match ciphertext", http.StatusBadRequest)
		return
	}

	payload, err := simplecrypto.Decrypt(a.aesKey, ciphertext)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(payload)
}
