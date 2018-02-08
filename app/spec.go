package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-units"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/dexec"
	"github.com/jakebailey/ua/pkg/docker/image"
	"github.com/jakebailey/ua/pkg/js"
	"github.com/jakebailey/ua/pkg/simplecrypto"
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

	imageID, containerID, err := a.specCreate(ctx, path, spec.Data)
	if err != nil {
		if err != errNoJS {
			return nil, err
		}

		imageID, containerID, err = a.specOldCreate(ctx, path, spec.Data)
		if err != nil {
			return nil, err
		}
	}

	instance := models.NewInstance()
	instance.ImageID = imageID
	instance.ContainerID = containerID
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

func (a *App) specOldCreate(ctx context.Context, assignmentPath string, specData interface{}) (imageID, containerID string, err error) {
	logger := ctxlog.FromContext(ctx)

	// Empty image and container names are easier to identify and cleanup if
	// needed, so just leave them blank.
	imageTag := ""
	containerName := ""

	truth := true

	containerConfig := container.Config{
		Tty:       true,
		OpenStdin: true,
		Image:     imageID,
	}
	hostConfig := container.HostConfig{
		Init:        &truth,
		NetworkMode: "none",
	}

	imageID, err = image.BuildLegacy(ctx, a.cli, assignmentPath, imageTag, specData)
	if err != nil {
		logger.Error("error building image",
			zap.Error(err),
		)
		return "", "", err
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

	c, err := a.cli.ContainerCreate(ctx, &containerConfig, &hostConfig, nil, containerName)
	if err != nil {
		logger.Error("error creating container",
			zap.Error(err),
		)
		return "", "", err
	}

	return imageID, c.ID, nil
}

var errNoJS = errors.New("no JS code found for assignment")

func (a *App) specCreate(ctx context.Context, assignmentPath string, specData interface{}) (imageID, containerID string, err error) {
	logger := ctxlog.FromContext(ctx)

	if _, err := os.Stat(filepath.Join(assignmentPath, "index.js")); err != nil {
		if !os.IsNotExist(err) {
			logger.Error("error trying to load index.js",
				zap.Error(err),
			)
			return "", "", err
		}

		return "", "", errNoJS
	}

	consoleOutput := &bytes.Buffer{}
	runtime := js.NewRuntime(&js.Options{
		Stdout:       consoleOutput,
		ModuleLoader: js.PathsModuleLoader(assignmentPath),
		FileReader:   js.PathsFileReader(assignmentPath),
	})
	defer runtime.Destroy()

	out := struct {
		ImageName  string
		Dockerfile string
		Init       *bool

		PostBuild []struct {
			Action     string
			User       string
			WorkingDir string

			// Exec action
			Cmd   []string
			Stdin *string

			// Write option
			Contents       string
			ContentsBase64 bool
			Filename       string
		}

		User       string
		Cmd        []string
		WorkingDir string
	}{}

	runtime.Set("__specData__", specData)
	if err := runtime.Run(ctx, "require('index.js').generate(__specData__);", &out); err != nil {
		logger.Error("javascript error",
			zap.Error(err),
			zap.String("console", consoleOutput.String()),
		)
		return "", "", err
	}

	// Empty image and container names are easier to identify and cleanup if
	// needed, so just leave them blank.
	imageTag := "TODO"
	containerName := ""

	switch {
	case out.ImageName != "":
		if err := a.cli.ImageTag(ctx, out.ImageName, imageTag); err != nil {
			if !strings.Contains(err.Error(), "No such image:") {
				return "", "", err
			}

			resp, err := a.cli.ImagePull(ctx, out.ImageName, types.ImagePullOptions{})
			if err != nil {
				return "", "", err
			}
			io.Copy(ioutil.Discard, resp)
			resp.Close()

			if err := a.cli.ImageTag(ctx, out.ImageName, imageTag); err != nil {
				return "", "", err
			}
		}

		imageID = imageTag

	case out.Dockerfile != "":
		contextPath := filepath.Join(assignmentPath, "context")

		imageID, err = image.Build(ctx, a.cli, imageTag, out.Dockerfile, contextPath)
		if err != nil {
			return "", "", err
		}

	default:
		logger.Error("not enough info to build image (image name, dockerfile, etc)")
		return "", "", errors.New("TODO: no way to build image")
	}

	containerConfig := &container.Config{
		Image:     imageID,
		OpenStdin: true,
		Cmd:       []string{"/bin/cat"},
	}
	hostConfig := &container.HostConfig{
		Init: out.Init,
	}

	c, err := a.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, containerName)
	if err != nil {
		return "", "", err
	}
	containerID = c.ID

	if err := a.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return "", "", err
	}

	// TODO: stop container on further errors

	for _, ac := range out.PostBuild {
		switch ac.Action {
		case "exec":
			ec := dexec.Config{
				User:       ac.User,
				Cmd:        ac.Cmd,
				WorkingDir: ac.WorkingDir,
				Stdout:     os.Stdout,
				Stderr:     os.Stderr,
			}

			if err := dexec.Exec(ctx, a.cli, containerID, ec); err != nil {
				return "", "", err
			}

		case "write":
			var r io.Reader = strings.NewReader(ac.Contents)
			if ac.ContentsBase64 {
				r = base64.NewDecoder(base64.StdEncoding, r)
			}

			ec := dexec.Config{
				User:       ac.User,
				Cmd:        []string{"dd", "of=" + ac.Filename},
				WorkingDir: ac.WorkingDir,
				Stdin:      r,
			}

			if err := dexec.Exec(ctx, a.cli, containerID, ec); err != nil {
				return "", "", err
			}

		default:
			logger.Warn("unknown action",
				zap.String("action", ac.Action),
			)
		}
	}

	if err := a.cli.NetworkDisconnect(ctx, "bridge", containerID, true); err != nil {
		logger.Error("error disconnecting network",
			zap.Error(err),
		)
	}

	if err := a.cli.ContainerStop(ctx, containerID, nil); err != nil {
		logger.Error("error disconnecting network",
			zap.Error(err),
		)
		return "", "", err
	}

	return imageID, containerID, nil
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
