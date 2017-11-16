package app

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-units"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/jakebailey/ua/ctxlog"
	"github.com/jakebailey/ua/image"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/simplecrypto"
	"github.com/jakebailey/ua/templates"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-kallax.v1"
)

var (
	nilULID = kallax.ULID(uuid.Nil)
)

func (a *App) routeSpec(r chi.Router) {
	r.Use(middleware.NoCache)

	if a.debug {
		r.Get("/", a.specGet)
	}

	r.Post("/", a.specPost)
	r.Post("/clean", a.specClean)
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

func (a *App) specProcessRequest(w http.ResponseWriter, r *http.Request) kallax.ULID {
	ctx := r.Context()
	logger := ctxlog.FromContext(ctx)

	defer r.Body.Close()
	payload, err := simplecrypto.DecodeJSONReader(a.aesKey, r.Body)
	if err != nil {
		logger.Warn("error decrypting payload",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusBadRequest)
		return nilULID
	}

	var req specPostRequest

	if err := json.Unmarshal(payload, &req); err != nil {
		logger.Warn("error decoding specPostRequest",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusBadRequest)
		return nilULID
	}

	if req.SpecID == "" {
		http.Error(w, "spec ID cannot be blank", http.StatusBadRequest)
		return nilULID
	}

	if req.AssignmentName == "" {
		http.Error(w, "assignment name cannot be blank", http.StatusBadRequest)
		return nilULID
	}

	specID, err := kallax.NewULIDFromText(req.SpecID)
	if err != nil {
		a.httpError(w, err.Error(), http.StatusBadRequest)
		return nilULID
	}

	if specID.IsEmpty() {
		http.Error(w, "spec ID cannot be all zero", http.StatusBadRequest)
		return nilULID
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
			return nilULID
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
			return nilULID
		}
	}

	return specID
}

func (a *App) specPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctxlog.FromContext(ctx)

	specID := a.specProcessRequest(w, r)
	if specID.IsEmpty() {
		return
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

	render.JSON(w, r, resp)
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

	pathSlice := []string{a.assignmentPath}
	pathSlice = append(pathSlice, strings.Split(spec.AssignmentName, ".")...)
	path := filepath.Join(pathSlice...)

	// Empty image and container names are easier to identify and cleanup if
	// needed, so just leave them blank.
	imageTag := ""
	containerName := ""

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
	}

	if initCmd, ok := image.GetLabel(ctx, a.cli, imageID, "ua.initCmd"); ok {
		containerConfig.Cmd = []string{"/dev/init", "-s", "--", "/bin/sh", "-c", initCmd}
	}

	if !a.disableLimits {
		hostConfig.Resources.CPUShares = 2
		hostConfig.Resources.Memory = 16 * units.MiB
		hostConfig.Resources.MemoryReservation = 4 * units.MiB
		hostConfig.StorageOpt = map[string]string{
			"size": "500M",
		}
	}

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

func (a *App) specClean(w http.ResponseWriter, r *http.Request) {
	specID := a.specProcessRequest(w, r)
	if specID.IsEmpty() {
		return
	}

	async := true
	switch r.URL.Query().Get("async") {
	case "false", "0":
		async = false
	}

	fn := func() {
		logger := ctxlog.FromRequest(r)
		ctx := ctxlog.WithLogger(context.Background(), logger)
		instanceQuery := models.NewInstanceQuery().FindBySpec(specID).FindByCleaned(false)
		a.cleanupInstancesByQuery(ctx, instanceQuery)
	}

	if async {
		go fn()
	} else {
		fn()
	}

	w.Write([]byte("ok"))
}
