package app

import (
	"context"
	"fmt"
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
	"github.com/jakebailey/ua/image"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/proxy"
	"go.uber.org/zap"
	kallax "gopkg.in/src-d/go-kallax.v1"
)

var termIDKey = &contextKey{"termID"}

func (a *App) routeTerm(r chi.Router) {
	r.Route("/{id}", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			fn := func(w http.ResponseWriter, r *http.Request) {
				IDStr := chi.URLParam(r, "id")
				id, err := kallax.NewULIDFromText(IDStr)
				if err != nil {
					a.httpError(w, err.Error(), http.StatusBadRequest)
					return
				}

				query := models.NewSpecQuery().FindByID(id)
				n, err := a.specStore.Count(query)
				if err != nil {
					a.httpError(w, err.Error(), http.StatusInternalServerError)
					return
				}

				if n == 0 {
					http.NotFound(w, r)
					return
				}

				ctx := r.Context()
				ctx = context.WithValue(ctx, termIDKey, id)
				r = r.WithContext(ctx)
				next.ServeHTTP(w, r)
			}
			return http.HandlerFunc(fn)
		})

		if a.debug {
			r.Get("/", a.termPage)
		}

		r.Get("/ws", a.termWS)
	})
}

func (a *App) termPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := ctx.Value(termIDKey).(kallax.ULID)
	fmt.Fprintf(w, "%v", id)
}

func (a *App) termWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctxlog.FromContext(ctx)

	specID := ctx.Value(termIDKey).(kallax.ULID)

	activeInstances := kallax.Eq(models.Schema.Instance.Active, true)
	specQuery := models.NewSpecQuery().FindByID(specID).WithInstances(activeInstances)

	spec, err := a.specStore.FindOne(specQuery)
	if err != nil {
		if err == kallax.ErrNotFound {
			http.NotFound(w, r)
		} else {
			logger.Error("error querying spec",
				zap.Error(err),
			)
			a.httpError(w, err.Error(), http.StatusInternalServerError)
		}
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

	go a.handleTerm(ctx, conn, spec)
}

func (a *App) handleTerm(ctx context.Context, conn net.Conn, spec *models.Spec) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := ctxlog.FromContext(ctx).With(zap.String("spec_id", spec.ID.String()))
	ctx = ctxlog.WithLogger(ctx, logger)

	var instance *models.Instance

	instancesLen := len(spec.Instances)
	if instancesLen == 0 {
		var err error
		instance, err = a.createInstance(ctx, spec)
		if err != nil {
			logger.Error("error creating instance",
				zap.Error(err),
			)
			return
		}
	} else {
		if instancesLen != 1 {
			logger.Warn("found multiple active instances, using most recently created",
				zap.Int("instances_len", instancesLen),
			)

			sort.Slice(spec.Instances, func(i, j int) bool {
				return spec.Instances[i].CreatedAt.After(spec.Instances[j].CreatedAt)
			})
		}
		instance = spec.Instances[0]
		// TODO: disconnect instance's existing client
	}

	logger = logger.With(zap.String("container_id", instance.ContainerID))
	ctx = ctxlog.WithLogger(ctx, logger)

	proxyConn := proxy.NewWSConn(conn)

	if err := proxy.Proxy(ctx, instance.ContainerID, proxyConn, a.cli); err != nil {
		logger.Error("error proxying container",
			zap.Error(err),
		)
	}

	a.stopInstance(ctx, instance)
}

func (a *App) createInstance(ctx context.Context, spec *models.Spec) (*models.Instance, error) {
	logger := ctxlog.FromContext(ctx)

	a.instanceMu.RLock()
	defer a.instanceMu.RUnlock()

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

	expiresAt := time.Now().Add(4 * time.Hour)

	instance := models.NewInstance()
	instance.ImageID = imageID
	instance.ContainerID = c.ID
	instance.ExpiresAt = &expiresAt
	instance.Active = true

	spec.Instances = append(spec.Instances, instance)
	if _, err := a.specStore.Update(spec); err != nil {
		logger.Error("error updating spec with new instance",
			zap.Error(err),
		)
		return nil, err
	}

	return instance, nil
}

func (a *App) stopInstance(ctx context.Context, instance *models.Instance) {
	logger := ctxlog.FromContext(ctx)

	a.instanceMu.RLock()
	defer a.instanceMu.RUnlock()

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
