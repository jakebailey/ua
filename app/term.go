package app

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi"
	"github.com/gobwas/ws"
	"github.com/jakebailey/ua/ctxlog"
	"github.com/jakebailey/ua/expire"
	"github.com/jakebailey/ua/image"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/proxy"
	"github.com/jakebailey/ua/templates"
	"go.uber.org/zap"
	kallax "gopkg.in/src-d/go-kallax.v1"
)

var termIDKey = &contextKey{"termID"}

func (a *App) routeTerm(r chi.Router) {
	r.Route("/{id}", func(r chi.Router) {
		if a.debug {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				wsURL := r.Host + r.RequestURI + "/ws"
				templates.WriteContainer(w, wsURL)
			})
		}

		r.Get("/ws", a.termWS)
	})
}

func (a *App) termWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctxlog.FromContext(ctx)

	idStr := chi.URLParam(r, "id")
	specID, err := kallax.NewULIDFromText(idStr)
	if err != nil {
		logger.Warn("error parsing specID",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	specQuery := models.NewSpecQuery().FindByID(specID)
	n, err := a.specStore.Count(specQuery)
	if err != nil {
		logger.Error("error querying spec",
			zap.Error(err),
		)
		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if n == 0 {
		http.NotFound(w, r)
		return
	}

	conn, _, _, err := ws.UpgradeHTTP(r, w, nil)
	if err != nil {
		// No need to write an error, UpgradeHTTP does this itself.
		logger.Error("error upgrading websocket",
			zap.Error(err),
		)
		return
	}

	ctx = context.Background()
	ctx = ctxlog.WithLogger(ctx, logger)

	a.wsWG.Add(1)
	go a.handleTerm(ctx, conn, specID)
}

func (a *App) handleTerm(ctx context.Context, conn net.Conn, specID kallax.ULID) {
	defer a.wsWG.Done()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := ctxlog.FromContext(ctx).With(zap.String("spec_id", specID.String()))
	ctx = ctxlog.WithLogger(ctx, logger)

	instance, err := a.getActiveInstance(ctx, specID)
	if err != nil {
		logger.Error("error getting active instance",
			zap.Error(err),
		)
		return
	}

	logger = logger.With(
		zap.String("instance_id", instance.ID.String()),
		zap.String("container_id", instance.ContainerID),
	)
	ctx = ctxlog.WithLogger(ctx, logger)

	var proxyConn proxy.Conn = proxy.NewWSConn(conn)

	token := a.wsManager.Acquire(
		instance.ID.String(),
		func() {
			logger.Debug("websocket expired")
			if err := proxyConn.Close(); err != nil {
				logger.Error("error closing connection on expiry",
					zap.Error(err),
				)
			}
		},
	)
	defer a.wsManager.Return(token)

	proxyConn = tokenProxyConn{
		Conn:  proxyConn,
		token: token,
	}

	if err := proxy.Proxy(ctx, instance.ContainerID, proxyConn, a.cli); err != nil {
		logger.Error("error proxying container",
			zap.Error(err),
		)
	}

	expiresAt := time.Now().Add(4 * time.Hour)
	instance.ExpiresAt = &expiresAt

	if _, err := a.instanceStore.Update(instance, models.Schema.Instance.ExpiresAt); err != nil {
		logger.Error("error adding ExpiresAt to instance",
			zap.Error(err),
		)
	}
}

func (a *App) getActiveInstance(ctx context.Context, specID kallax.ULID) (*models.Instance, error) {
	logger := ctxlog.FromContext(ctx)

	var instance *models.Instance

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

	instance = instances[0]
	logger.Debug("reusing active instance")

	instance.ExpiresAt = nil
	if _, err := a.instanceStore.Update(instance, models.Schema.Instance.ExpiresAt); err != nil {
		logger.Error("error disabling expiry for instance",
			zap.Error(err),
		)
		return nil, err
	}

	return instance, nil
}

func (a *App) createInstance(ctx context.Context, specID kallax.ULID) (*models.Instance, error) {
	logger := ctxlog.FromContext(ctx)

	specQuery := models.NewSpecQuery().FindByID(specID).Select(
		models.Schema.Spec.AssignmentName,
		models.Schema.Spec.Seed,
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

	// TODO: Manage networking, cpu, memory
	c, err := a.cli.ContainerCreate(ctx, &container.Config{
		Tty:       true,
		OpenStdin: true,
		Image:     imageID,
	}, &container.HostConfig{
		Init: &truth,
	}, nil, containerName)
	if err != nil {
		logger.Error("error creating container",
			zap.Error(err),
		)
		return nil, err
	}

	if err := a.cli.ContainerStart(ctx, c.ID, types.ContainerStartOptions{}); err != nil {
		logger.Error("error starting container",
			zap.Error(err),
		)
		return nil, err
	}

	instance := models.NewInstance()
	instance.ImageID = imageID
	instance.ContainerID = c.ID
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

func (a *App) stopInstance(ctx context.Context, instance *models.Instance) {
	logger := ctxlog.FromContext(ctx)

	if err := a.cli.ContainerStop(ctx, instance.ContainerID, nil); err != nil {
		logger.Error("error stopping container",
			zap.Error(err),
		)
	}

	logger.Info("stopped container")

	if err := a.cli.ContainerRemove(ctx, instance.ContainerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
	}); err != nil {
		logger.Error("error removing container",
			zap.Error(err),
		)
	}

	logger.Info("removed container")

	instance.Active = false

	if _, err := a.instanceStore.Update(instance, models.Schema.Instance.Active); err != nil {
		logger.Error("error marking instance as inactive in database",
			zap.Error(err),
		)
	}
}

type tokenProxyConn struct {
	proxy.Conn
	token *expire.Token
}

var _ proxy.Conn = tokenProxyConn{}

func (t tokenProxyConn) ReadJSON(v interface{}) error {
	t.token.Update()
	return t.Conn.ReadJSON(v)
}

func (t tokenProxyConn) WriteJSON(v interface{}) error {
	t.token.Update()
	return t.Conn.WriteJSON(v)
}
