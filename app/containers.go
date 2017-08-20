package app

import (
	"context"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/api/types"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gobwas/ws"
	"github.com/jakebailey/ua/proxy"
	"github.com/jakebailey/ua/templates"
	"github.com/rs/zerolog"
	"github.com/satori/go.uuid"
)

var (
	containerUUIDKey = &contextKey{"containerUUID"}
)

func (a *App) routeContainers(r chi.Router) {
	r.Use(middleware.NoCache)

	if a.debug {
		r.Get("/", a.containersList)
	}

	r.Route("/{uuid}", func(r chi.Router) {
		r.Use(func(h http.Handler) http.Handler {
			fn := func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				log := zerolog.Ctx(ctx)

				s := chi.URLParam(r, "uuid")

				u, err := uuid.FromString(s)
				if err != nil {
					log.Error().Err(err).Msg("error parsing UUID")
					a.httpError(w, err.Error(), http.StatusInternalServerError)
					return
				}

				a.activeMu.RLock()
				_, ok := a.active[u]
				a.activeMu.RUnlock()

				if !ok {
					http.NotFound(w, r)
					return
				}

				ctx = context.WithValue(ctx, containerUUIDKey, u)
				r = r.WithContext(ctx)

				h.ServeHTTP(w, r)
			}
			return http.HandlerFunc(fn)
		})

		if a.debug {
			r.Get("/", a.containerPage)
		}

		r.Get("/ws", a.containerAttach)
	})
}

func (a *App) containersList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zerolog.Ctx(ctx)

	containers, err := a.cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("error listing containers")
		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	spew.Fdump(w, containers)
}

func (a *App) containerPage(w http.ResponseWriter, r *http.Request) {
	wsURL := r.Host + r.RequestURI + "/ws"
	templates.WriteContainer(w, wsURL)
}

func (a *App) containerAttach(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zerolog.Ctx(ctx)

	u := ctx.Value(containerUUIDKey).(uuid.UUID)

	a.activeMu.Lock()
	id := a.active[u]
	delete(a.active, u)
	a.activeMu.Unlock()

	log = log.With().Str("container_id", id).Str("container_uuid", u.String()).Logger()
	ctx = log.WithContext(ctx)

	conn, _, _, err := ws.UpgradeHTTP(r, w, nil)
	if err != nil {
		// No need to write an error, UpgradeHTTP does this itself.
		log.Error().Err(err).Msg("error upgrading to websocket")
		return
	}

	go func() {
		defer conn.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := a.cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
			log.Error().Err(err).Msg("error starting container")
			return
		}

		log.Info().Str("container_id", id).Msg("started container")

		proxyConn := proxy.NewWSConn(conn)

		if err := proxy.Proxy(ctx, id, proxyConn, a.cli); err != nil {
			log.Error().Err(err).Msg("error proxying container")
		}

		if err := a.cli.ContainerStop(ctx, id, nil); err != nil {
			log.Error().Err(err).Msg("error stopping container")
		}

		log.Info().Msg("stopped container")

		if err := a.cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
			RemoveVolumes: true,
		}); err != nil {
			log.Error().Err(err).Msg("error removing container")
		}

		log.Info().Msg("removed container")
	}()
}
