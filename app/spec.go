package app

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/jakebailey/ua/ctxlog"
	"github.com/jakebailey/ua/image"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/simplecrypto"
	"github.com/jakebailey/ua/templates"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-kallax.v1"
)

func (a *App) routeSpec(r chi.Router) {
	r.Use(middleware.NoCache)

	if a.debug {
		r.Get("/", a.specGet)
	}

	r.Post("/", a.specPost)
}

func (a *App) specGet(w http.ResponseWriter, r *http.Request) {
	specID := kallax.NewULID().String()
	templates.WriteSpec(w, specID)
}

type specPostRequest struct {
	SpecID         string      `json:"specID"`
	AssignmentName string      `json:"assignmentName"`
	Data           interface{} `json:"data"`
}

type specPostResponse struct {
	InstanceID string `json:"instanceID"`
}

func (a *App) specPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctxlog.FromContext(ctx)

	defer r.Body.Close()
	payload, err := simplecrypto.DecodeJSONReader(a.aesKey, r.Body)
	if err != nil {
		logger.Warn("error decrypting payload",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req specPostRequest

	if err := json.Unmarshal(payload, &req); err != nil {
		logger.Warn("error decoding specPostRequest",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.SpecID == "" {
		http.Error(w, "spec ID cannot be blank", http.StatusBadRequest)
		return
	}

	if req.AssignmentName == "" {
		http.Error(w, "assignment name cannot be blank", http.StatusBadRequest)
		return
	}

	specID, err := kallax.NewULIDFromText(req.SpecID)
	if err != nil {
		a.httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	specQuery := models.NewSpecQuery().FindByID(specID)

	spec, err := a.specStore.FindOne(specQuery)
	if err != nil {
		if err != kallax.ErrNotFound {
			logger.Error("error querying for spec",
				zap.Error(err),
				zap.Any("spec_id", specID.String()),
			)
			a.httpError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		spec = &models.Spec{
			ID:             specID,
			AssignmentName: req.AssignmentName,
			Data:           req.Data,
		}

		if err := a.specStore.Insert(spec); err != nil {
			logger.Error("error inserting spec",
				zap.Error(err),
				zap.Any("spec_id", specID.String()),
			)
			a.httpError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	instance, err := a.getActiveInstance(ctx, specID)
	if err != nil {
		logger.Error("error getting active instance",
			zap.Error(err),
			zap.Any("spec_id", specID.String()),
		)
		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := &specPostResponse{
		InstanceID: instance.ID.String(),
	}

	render.Respond(w, r, resp)
}

func (a *App) getActiveInstance(ctx context.Context, specID kallax.ULID) (*models.Instance, error) {
	logger := ctxlog.FromContext(ctx)

	instanceQuery := models.NewInstanceQuery().FindBySpec(specID).FindByActive(true)
	instances, err := a.instanceStore.FindAll(instanceQuery)
	if err != nil {
		logger.Error("error querying for instances",
			zap.Error(err),
		)
		return nil, err
	}

	instancesLen := len(instances)
	if instancesLen == 0 {
		logger.Debug("no active instance found, creating a new instance")
		return a.createInstance(ctx, specID)
	}

	if instancesLen != 1 {
		logger.Warn("found multiple active instances, using most recently created",
			zap.Int("instances_len", instancesLen),
		)

		sort.Slice(instances, func(i, j int) bool {
			return instances[i].CreatedAt.After(instances[j].CreatedAt)
		})
	}

	logger.Debug("reusing active instance")

	return instances[0], nil
}

func (a *App) createInstance(ctx context.Context, specID kallax.ULID) (*models.Instance, error) {
	logger := ctxlog.FromContext(ctx)

	specQuery := models.NewSpecQuery().FindByID(specID).Select(
		models.Schema.Spec.AssignmentName,
		models.Schema.Spec.Data,
	)
	spec, err := a.specStore.FindOne(specQuery)
	if err != nil {
		logger.Error("error querying spec for build info",
			zap.Error(err),
		)
		return nil, err
	}

	path := filepath.Join(a.assignmentPath, spec.AssignmentName)
	imageTag := ""      // TODO: pick tag
	containerName := "" // TODO: pick name

	// TODO: define build data struct with rand/data
	imageID, err := image.Build(ctx, a.cli, path, imageTag, spec.Data)
	if err != nil {
		logger.Error("error building image",
			zap.Error(err),
		)
		return nil, err
	}

	truth := true

	containerConfig := &container.Config{
		Tty:       true,
		OpenStdin: true,
		Image:     imageID,
	}
	hostConfig := &container.HostConfig{
		Init:        &truth,
		NetworkMode: "none",
		Resources: container.Resources{
			CPUShares: 2,
		},
	}

	// TODO: Manage networking, cpu, memory
	c, err := a.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, containerName)
	if err != nil {
		logger.Error("error creating container",
			zap.Error(err),
		)
		return nil, err
	}

	instance := models.NewInstance()
	instance.ImageID = imageID
	instance.ContainerID = c.ID
	instance.ExpiresAt = a.instanceExpireTime()
	instance.Active = true

	if err := a.specStore.Transaction(func(specStore *models.SpecStore) error {
		specQuery := models.NewSpecQuery().FindByID(specID)
		spec, err := specStore.FindOne(specQuery)
		if err != nil {
			return err
		}

		spec.Instances = append(spec.Instances, instance)

		_, err = specStore.Update(spec)
		return err
	}); err != nil {
		logger.Error("error inserting new instance",
			zap.Error(err),
		)
		return nil, err
	}

	return instance, nil
}
