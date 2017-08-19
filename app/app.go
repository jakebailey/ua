package app

import (
	"net/http"
	"os"
	"sync"

	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
	"github.com/rs/zerolog"
	"github.com/satori/go.uuid"
)

var (
	// DefaultAssignmentPath is the path assignments are stored in. If relative,
	// then this will be relative to the current working directory.
	DefaultAssignmentPath = "assignments"
	// DefaultLogger is the default zerolog Logger that will be used. It
	// defaults to outputting to stderr in JSON format.
	DefaultLogger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	// DefaultStaticPath is the path to the static elements served at /static
	// by the app.
	DefaultStaticPath = "static"
)

type App struct {
	debug          bool
	assignmentPath string
	staticPath     string

	router chi.Router
	log    zerolog.Logger
	cli    client.CommonAPIClient

	active   map[uuid.UUID]string
	activeMu sync.RWMutex
}

func NewApp(cli client.CommonAPIClient, options ...Option) *App {
	a := &App{
		assignmentPath: DefaultAssignmentPath,
		log:            DefaultLogger,
		staticPath:     DefaultStaticPath,
		cli:            cli,
		active:         make(map[uuid.UUID]string),
	}

	for _, o := range options {
		o(a)
	}

	a.route()

	return a
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}

type Option func(*App)

func Debug() Option {
	return func(a *App) {
		a.debug = true
	}
}

func AssignmentPath(path string) Option {
	return func(a *App) {
		a.assignmentPath = path
	}
}

func Logger(log zerolog.Logger) Option {
	return func(a *App) {
		a.log = log
	}
}

func ConsoleLogger() Option {
	return func(a *App) {
		a.log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	}
}
