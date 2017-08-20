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
	"github.com/jakebailey/ua/image"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/proxy"
	"github.com/rs/zerolog"
	kallax "gopkg.in/src-d/go-kallax.v1"
)

var (
	termIDKey = &contextKey{"termID"}
)

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
	log := zerolog.Ctx(ctx)

	specID := ctx.Value(termIDKey).(kallax.ULID)

	activeInstances := kallax.Eq(models.Schema.Instance.Active, true)
	specQuery := models.NewSpecQuery().FindByID(specID).WithInstances(activeInstances)

	spec, err := a.specStore.FindOne(specQuery)
	if err != nil {
		if err == kallax.ErrNotFound {
			http.NotFound(w, r)
		} else {
			log.Error().Err(err).Msg("error querying spec")
			a.httpError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	conn, _, _, err := ws.UpgradeHTTP(r, w, nil)
	if err != nil {
		// No need to write an error, UpgradeHTTP does this itself.
		log.Error().Err(err).Msg("error upgrading to websocket")
		return
	}

	go a.handleTerm(conn, spec)
}

func (a *App) handleTerm(conn net.Conn, spec *models.Spec) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := a.log.With().Str("spec_id", spec.ID.String()).Logger()
	ctx = log.WithContext(ctx)

	var instance *models.Instance

	instancesLen := len(spec.Instances)
	if instancesLen == 0 {
		var err error
		instance, err = a.createInstance(ctx, spec)
		if err != nil {
			log.Error().Err(err).Msg("error creating instance")
			return
		}
	} else {
		if instancesLen != 1 {
			log.Warn().
				Int("instances_len", instancesLen).
				Msg("found multiple active instances, using most recently created")

			sort.Slice(spec.Instances, func(i, j int) bool {
				return spec.Instances[i].CreatedAt.After(spec.Instances[j].CreatedAt)
			})
		}
		instance = spec.Instances[0]
		// TODO: disconnect instance's existing client
	}

	log = log.With().Str("container_id", instance.ContainerID).Logger()
	ctx = log.WithContext(ctx)

	proxyConn := proxy.NewWSConn(conn)

	if err := proxy.Proxy(ctx, instance.ContainerID, proxyConn, a.cli); err != nil {
		log.Error().Err(err).Msg("error proxying container")
	}

	a.stopInstance(ctx, instance)
}

func (a *App) createInstance(ctx context.Context, spec *models.Spec) (*models.Instance, error) {
	log := zerolog.Ctx(ctx)

	a.instanceMu.RLock()
	defer a.instanceMu.RUnlock()

	path := filepath.Join(a.assignmentPath, spec.AssignmentName)
	imageTag := ""      // TODO: pick tag
	containerName := "" // TODO: pick name

	// TODO: define build data struct with rand/data
	imageID, err := image.Build(ctx, a.cli, path, imageTag, spec.Data)
	if err != nil {
		log.Error().Err(err).Msg("error building image")
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
		log.Error().Err(err).Msg("error creating container")
		return nil, err
	}

	if err := a.cli.ContainerStart(ctx, c.ID, types.ContainerStartOptions{}); err != nil {
		log.Error().Err(err).Msg("error starting container")
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
		log.Error().Err(err).Msg("error updating spec with new instance")
		return nil, err
	}

	return instance, nil
}

func (a *App) stopInstance(ctx context.Context, instance *models.Instance) {
	log := zerolog.Ctx(ctx)

	a.instanceMu.RLock()
	defer a.instanceMu.RUnlock()

	if err := a.cli.ContainerStop(ctx, instance.ContainerID, nil); err != nil {
		log.Error().Err(err).Msg("error stopping container")
	}

	log.Info().Msg("stopped container")

	if err := a.cli.ContainerRemove(ctx, instance.ContainerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
	}); err != nil {
		log.Error().Err(err).Msg("error removing container")
	}

	log.Info().Msg("removed container")

	instance.Active = false

	if _, err := a.instanceStore.Update(instance, models.Schema.Instance.Active); err != nil {
		log.Error().Err(err).Msg("error marking instance as inactive in database")
	}
}
