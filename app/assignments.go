package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/satori/go.uuid"

	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi"
	"github.com/jakebailey/ua/image"
	"github.com/jakebailey/ua/templates"
)

const (
	assignmentPathKey = contextKey("assignmentPath")
)

func (a *App) routeAssignments(r chi.Router) {
	if a.debug {
		r.Get("/", a.assignmentsList)
	}

	r.Route("/{name}", func(r chi.Router) {
		r.Use(func(h http.Handler) http.Handler {
			fn := func(w http.ResponseWriter, r *http.Request) {
				name := chi.URLParam(r, "name")
				path := filepath.Join(a.assignmentPath, name)

				stat, err := os.Stat(path)
				if err != nil {
					http.NotFound(w, r)
					if !os.IsNotExist(err) {
						log.Println(err)
					}
					return
				}

				if !stat.IsDir() {
					http.NotFound(w, r)
					return
				}

				ctx := r.Context()
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
		http.Error(w, err.Error(), 500)
		return
	}

	truth := true

	c, err := a.cli.ContainerCreate(ctx, &container.Config{
		Tty:       true,
		OpenStdin: true,
		Image:     id,
	}, &container.HostConfig{
		Init: &truth,
	}, nil, containerName)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	log.Printf("%v: created", c.ID[:10])

	a.activeMu.Lock()
	a.active[u] = c.ID
	a.activeMu.Unlock()

	templates.WriteAssignments(w, u.String())
}
