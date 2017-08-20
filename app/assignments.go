package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jakebailey/ua/image"
	"github.com/jakebailey/ua/templates"
	"github.com/rs/zerolog"
	"github.com/satori/go.uuid"
)

var (
	assignmentPathKey = &contextKey{"assignmentPath"}
)

func (a *App) routeAssignments(r chi.Router) {
	r.Use(middleware.NoCache)

	if a.debug {
		r.Get("/", a.assignmentsList)
	}

	r.Route("/{name}", func(r chi.Router) {
		r.Use(func(h http.Handler) http.Handler {
			fn := func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				log := zerolog.Ctx(ctx)

				name := chi.URLParam(r, "name")
				path := filepath.Join(a.assignmentPath, name)

				stat, err := os.Stat(path)
				if err != nil {
					http.NotFound(w, r)
					if !os.IsNotExist(err) {
						log.Error().
							Err(err).
							Msg("unexpected error while stat-ing assignment dir")
					}
					return
				}

				if !stat.IsDir() {
					http.NotFound(w, r)
					return
				}

				ctx = context.WithValue(ctx, assignmentPathKey, path)
				r = r.WithContext(ctx)

				h.ServeHTTP(w, r)
			}
			return http.HandlerFunc(fn)
		})

		r.Get("/", a.assignmentBuild)
	})
}

func (a *App) assignmentsList(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "todo")
}

func (a *App) assignmentBuild(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := zerolog.Ctx(ctx)

	path := ctx.Value(assignmentPathKey).(string)
	u := uuid.NewV4()

	token := fmt.Sprintf("%s-%d", chi.URLParam(r, "name"), time.Now().Unix())
	imageTag := token + "-image"
	containerName := token + "-container"

	data := map[string]interface{}{
		"NetID": "jbbaile2",
		"Now":   time.Now(),
	}

	id, err := image.Build(ctx, a.cli, path, imageTag, data)
	if err != nil {
		log.Error().Err(err).Msg("error building image")

		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("image_id", id).
		Str("image_tag", imageTag).
		Msg("built image")

	truth := true

	c, err := a.cli.ContainerCreate(ctx, &container.Config{
		Tty:       true,
		OpenStdin: true,
		Image:     id,
	}, &container.HostConfig{
		Init: &truth,
	}, nil, containerName)
	if err != nil {
		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("container_id", c.ID).
		Str("container_name", containerName).
		Msg("created container")

	a.activeMu.Lock()
	a.active[u] = c.ID
	a.activeMu.Unlock()

	templates.WriteAssignments(w, u.String())
}
