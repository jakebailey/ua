package app

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/jakebailey/ua/models"
	"github.com/rs/zerolog/hlog"
)

func (a *App) routeSpecs(r chi.Router) {
	r.Use(middleware.NoCache)
	r.Post("/", a.specPost)
}

type specPostRequest struct {
	AssignmentName string      `json:"assignmentName"`
	Seed           string      `json:"seed"`
	Data           interface{} `json:"data"`
}

type specPostResponse struct {
	ID string `json:"id"`
}

func (a *App) specPost(w http.ResponseWriter, r *http.Request) {
	log := hlog.FromRequest(r)

	var req specPostRequest

	if err := render.Decode(r, &req); err != nil {
		log.Warn().Err(err).Msg("error decoding specPostRequest")
		a.httpError(w, err.Error(), http.StatusBadRequest)
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

	spec := models.NewSpec()
	spec.AssignmentName = req.AssignmentName
	spec.Seed = req.Seed
	spec.Data = req.Data

	if err := a.specStore.Insert(spec); err != nil {
		log.Error().Err(err).Interface("spec", spec).Msg("error inserting new spec")
		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := specPostResponse{
		ID: spec.ID.String(),
	}

	render.Respond(w, r, resp)
}
