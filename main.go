package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jakebailey/ua/app"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

func main() {
	spew.Config.Indent = "    "
	spew.Config.ContinueOnMethod = true

	addr := ":8000"

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
		Timestamp().
		Str("addr", addr).
		Logger()

	r := chi.NewRouter()

	r.Use(hlog.NewHandler(log))
	r.Use(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Msg("http request")
	}))
	r.Use(hlog.RequestIDHandler("req_id", "Request-Id"))

	r.Use(func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					log := hlog.FromRequest(r)

					stack := string(debug.Stack())

					log.Error().
						Interface("panic_value", rvr).
						Str("stack", stack).
						Msg("PANIC")

					http.Error(w, stack, http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	})

	r.Use(middleware.CloseNotify)
	r.Use(middleware.Heartbeat("/ping"))

	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	a := app.NewApp(cli, app.Debug())
	a.Route(r)

	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, "static")
	r.Group(func(r chi.Router) {
		r.Use(middleware.DefaultCompress)
		FileServer(r, "/static", http.Dir(filesDir))
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Info().Msg("starting server")
		if err := srv.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("ListenAndServe error")
		}
	}()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	<-stopChan
	log.Info().Msg("shutting down")

	ctx, canc := context.WithTimeout(context.Background(), 5*time.Second)
	defer canc()
	srv.Shutdown(ctx)
}

// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}
