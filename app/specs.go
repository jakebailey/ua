package app

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/hlog"
	uuid "github.com/satori/go.uuid"
)

func (a *App) routeSpecs(r chi.Router) {
	r.Use(middleware.NoCache)
}

type specPostRequest struct {
	AssignmentName string      `json:"assignment_name"`
	Seed           string      `json:"seed"`
	Data           interface{} `json:"data"`
}

type specPostResponse struct {
	UUID uuid.UUID `json:"uuid"`
}

func (a *App) specPost(w http.ResponseWriter, r *http.Request) {
	log := hlog.FromRequest(r)

	var req specPostRequest

	if err := render.Decode(r, &req); err != nil {
		log.Warn().Err(err).Msg("error decoding specPostRequest")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if req.AssignmentName == "" {
		http.Error(w, "assignment name cannot be blank", http.StatusBadRequest)
		return
	}

	if req.Seed == "" {
		http.Error(w, "seed cannot be blank", http.StatusBadRequest)
		return
	}

	// TODO: the rest of this function
}
