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
	"github.com/jakebailey/ua/ctxlog"
	"github.com/jakebailey/ua/image"
	"github.com/jakebailey/ua/templates"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
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
				logger := ctxlog.FromContext(ctx)

				name := chi.URLParam(r, "name")
				path := filepath.Join(a.assignmentPath, name)

				stat, err := os.Stat(path)
				if err != nil {
					http.NotFound(w, r)
					if !os.IsNotExist(err) {
						logger.Error("unexpected error while stat-ing assignment dir",
							zap.Error(err),
						)
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
	logger := ctxlog.FromContext(ctx)

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
		logger.Error("error building image")

		a.httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info("built image",
		zap.String("image_id", id),
		zap.String("image_tag", imageTag),
	)

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

	logger.Info("created container",
		zap.String("container_id", c.ID),
		zap.String("container_name", containerName),
	)

	a.activeMu.Lock()
	a.active[u] = c.ID
	a.activeMu.Unlock()

	templates.WriteAssignments(w, u.String())
}
