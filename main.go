package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

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

	r.Route("/docker", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
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

			log.Printf("created %v", c.ID)

			templates.WriteDocker(w, c.ID)
		})

		r.Get("/:id/ws", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			id := chi.URLParam(r, "id")

			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}
			defer conn.Close()

			if err := cli.ContainerStart(r.Context(), id, types.ContainerStartOptions{}); err != nil {
				log.Println(err)
				return
			}

			log.Printf("%v: started", id[:10])

			if err := ProxyContainer(ctx, id, cli, conn); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), 500)
			}

			if err := cli.ContainerStop(r.Context(), id, nil); err != nil {
				log.Println(err)
			}

			log.Printf("%v: stopped", id[:10])

			if err := cli.ContainerRemove(r.Context(), id, types.ContainerRemoveOptions{
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

	http.ListenAndServe(":8000", r)
}
