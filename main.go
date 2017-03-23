package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	"github.com/jakebailey/ua/templates"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // REMOVE ME
}

//go:generate qtc -dir=templates

var (
	createdContainers = make(map[string]bool)
	createdMu         sync.RWMutex
)

func addCreatedContainer(id string) {
	createdMu.Lock()
	defer createdMu.Unlock()

	createdContainers[id] = true
}

func checkCreatedContainer(id string) bool {
	createdMu.RLock()
	defer createdMu.RUnlock()

	return createdContainers[id]
}

func removeCreatedContainer(id string) bool {
	createdMu.Lock()
	defer createdMu.Unlock()

	if !createdContainers[id] {
		return false
	}

	delete(createdContainers, id)
	return true
}

func main() {
	spew.Config.Indent = "    "
	spew.Config.ContinueOnMethod = true
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CloseNotify)
	r.Use(middleware.NoCache)
	r.Use(middleware.Heartbeat("/ping"))

	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		images, err := cli.ImageList(r.Context(), types.ImageListOptions{})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		spew.Fdump(w, images)
	})

	r.Get("/assignments/:name", func(w http.ResponseWriter, r *http.Request) {
		c, err := cli.ContainerCreate(r.Context(), &container.Config{
			Tty:       true,
			OpenStdin: true,
			Cmd:       []string{"/bin/bash"},
			Image:     "dock0/arch",
		}, &container.HostConfig{
			Init: func(b bool) *bool { return &b }(true),
		}, nil, "")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

		log.Printf("%v: created", c.ID[:10])

		addCreatedContainer(c.ID)

		templates.WriteAssignments(w, c.ID)
	})

	r.Route("/docker/:id", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			if !checkCreatedContainer(id) {
				http.NotFound(w, r)
				return
			}

			templates.WriteDocker(w, id)
		})

		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			id := chi.URLParam(r, "id")

			if !removeCreatedContainer(id) {
				http.NotFound(w, r)
				return
			}

			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}
			defer conn.Close()

			if err := cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
				log.Println(err)
				return
			}

			log.Printf("%v: started", id[:10])

			if err := ProxyContainer(ctx, id, cli, conn); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), 500)
			}

			if err := cli.ContainerStop(ctx, id, nil); err != nil {
				log.Println(err)
			}

			log.Printf("%v: stopped", id[:10])

			if err := cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
				RemoveVolumes: true,
			}); err != nil {
				log.Println(err)
			}

			log.Printf("%v: removed", id[:10])
		})
	})

	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, "static")
	r.Group(func(r chi.Router) {
		r.Use(middleware.DefaultCompress)
		r.(*chi.Mux).FileServer("/static", http.Dir(filesDir))
	})

	// srv := &http.Server{
	// 	Addr:    ":8000",
	// 	Handler: r,
	// }

	http.ListenAndServe(":8000", r)
}
