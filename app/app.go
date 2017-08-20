package app

import (
	"context"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
	"github.com/rs/zerolog"
	"github.com/satori/go.uuid"
)

var (
	// DefaultAddr is the default address where app will run its HTTP server.
	DefaultAddr = ":8000"
	// DefaultAssignmentPath is the path assignments are stored in. If relative,
	// then this will be relative to the current working directory.
	DefaultAssignmentPath = "assignments"
	// DefaultLogger is the default zerolog Logger that will be used. It
	// defaults to logging nothing.
	DefaultLogger = zerolog.New(ioutil.Discard).Level(zerolog.Disabled)
	// DefaultStaticPath is the path to the static elements served at /static
	// by the app.
	DefaultStaticPath = "static"
	// DefaultSpew is the spew configuration used in various debugging
	// endpoints.
	DefaultSpew = &spew.ConfigState{Indent: "    ", ContinueOnMethod: true}
)

type App struct {
	debug bool

	addr           string
	assignmentPath string
	staticPath     string

	router chi.Router
	srv    *http.Server

	log  zerolog.Logger
	spew *spew.ConfigState

	cli      client.CommonAPIClient
	cliClose func() error

	active   map[uuid.UUID]string
	activeMu sync.RWMutex
}

func NewApp(options ...Option) *App {
	a := &App{
		addr:           DefaultAddr,
		assignmentPath: DefaultAssignmentPath,
		log:            DefaultLogger,
		staticPath:     DefaultStaticPath,
		spew:           DefaultSpew,
		active:         make(map[uuid.UUID]string),
	}

	for _, o := range options {
		o(a)
	}

	a.route()

	// Ensure logger has a timestamp. This is a no-op if the logger already
	// has the timestamp enabled.
	a.log = a.log.With().Timestamp().Logger()

	return a
}

type Option func(*App)

func Debug(debug bool) Option {
	return func(a *App) {
		a.debug = debug
	}
}

func Addr(addr string) Option {
	return func(a *App) {
		a.addr = addr
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

func SpewConfig(c *spew.ConfigState) Option {
	return func(a *App) {
		a.spew = c
	}
}

func DockerClient(cli client.CommonAPIClient, closeFunc func() error) Option {
	return func(a *App) {
		a.cli = cli
		a.cliClose = closeFunc
	}
}

func (a *App) Run() error {
	if a.cli == nil {
		cli, err := client.NewEnvClient()
		if err != nil {
			return err
		}

		a.cli = cli
		a.cliClose = cli.Close
	}
	defer func() {
		if a.cliClose == nil {
			return
		}

		if err := a.cliClose(); err != nil {
			a.log.Error().Err(err).Msg("error closing docker client")
		}
	}()

	// Sanity check Docker client
	_, err := a.cli.Info(context.Background())
	if err != nil {
		return err
	}

	a.srv = &http.Server{
		Addr:    a.addr,
		Handler: a.router,
	}

	a.log.Info().Str("addr", a.addr).Msg("starting http server")
	return a.srv.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	a.log.Info().Str("addr", a.addr).Msg("shutting down http server")
	return a.srv.Shutdown(ctx)
}
