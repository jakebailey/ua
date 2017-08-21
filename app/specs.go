package app

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/jakebailey/ua/ctxlog"
	"github.com/jakebailey/ua/models"
	"go.uber.org/zap"
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
	logger := ctxlog.FromRequest(r)

	var req specPostRequest

	if err := render.Decode(r, &req); err != nil {
		logger.Warn("error decoding specPostRequest",
			zap.Error(err),
		)
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
		logger.Error("error inserting new spec",
			zap.Error(err),
			zap.Any("spec", spec),
		)

		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := specPostResponse{
		ID: spec.ID.String(),
	}

	render.Respond(w, r, resp)
}
