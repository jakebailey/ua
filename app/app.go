package app

import (
	"context"
	"database/sql"
	"net/http"
	"sync"

	_ "github.com/lib/pq" // postgresql driver

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
	"github.com/jakebailey/ua/models"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
)

var (
	// DefaultAddr is the default address where app will run its HTTP server.
	DefaultAddr = ":8000"
	// DefaultAssignmentPath is the path assignments are stored in. If relative,
	// then this will be relative to the current working directory.
	DefaultAssignmentPath = "assignments"
	// DefaultLogger is the default zerolog Logger that will be used. It
	// defaults to logging nothing.
	DefaultLogger = zap.NewNop()
	// DefaultStaticPath is the path to the static elements served at /static
	// by the app.
	DefaultStaticPath = "static"
	// DefaultSpew is the spew configuration used in various debugging
	// endpoints.
	DefaultSpew = &spew.ConfigState{Indent: "    ", ContinueOnMethod: true}
)

// App is the main application for uAssign.
type App struct {
	debug bool

	addr           string
	assignmentPath string
	staticPath     string

	router chi.Router
	srv    *http.Server

	logger *zap.Logger
	spew   *spew.ConfigState

	cli      client.CommonAPIClient
	cliClose func() error

	dbString      string
	db            *sql.DB
	specStore     *models.SpecStore
	instanceStore *models.InstanceStore

	active   map[uuid.UUID]string
	activeMu sync.RWMutex

	instanceMu sync.RWMutex
}

// NewApp creates a new app, with an optional list of options.
// This function does not open any connections, only setting up the app before
// Run is called.
func NewApp(dbString string, options ...Option) *App {
	a := &App{
		addr:           DefaultAddr,
		assignmentPath: DefaultAssignmentPath,
		logger:         DefaultLogger,
		staticPath:     DefaultStaticPath,
		spew:           DefaultSpew,
		dbString:       dbString,
		active:         make(map[uuid.UUID]string),
	}

	for _, o := range options {
		o(a)
	}

	a.route()

	return a
}

// Option is a function that runs on an App to configure it.
type Option func(*App)

// Debug sets the debug mode. The default is false.
func Debug(debug bool) Option {
	return func(a *App) {
		a.debug = debug
	}
}

// Addr sets the address of the app's HTTP server. If not provided,
// DefaultAddr is used.
func Addr(addr string) Option {
	return func(a *App) {
		a.addr = addr
	}
}

// AssignmentPath sets the path for the assignment directory. If not
// provided, DefaultAssignmentPath is used.
func AssignmentPath(path string) Option {
	return func(a *App) {
		a.assignmentPath = path
	}
}

// Logger sets the logger used within the app. If not provided,
// DefaultLogger is used.
func Logger(logger *zap.Logger) Option {
	return func(a *App) {
		a.logger = logger
	}
}

// SpewConfig sets the spew config state used for various debugging
// endpoints in the app. If not provided, DefaultSpew is used.
func SpewConfig(c *spew.ConfigState) Option {
	if c == nil {
		panic("app: spew ConfigState cannot be nil")
	}

	return func(a *App) {
		a.spew = c
	}
}

// DockerClient sets the docker client used in the app. If closeFunc is not
// nil, then it will be called when the app closes.
func DockerClient(cli client.CommonAPIClient, closeFunc func() error) Option {
	return func(a *App) {
		a.cli = cli
		a.cliClose = closeFunc
	}
}

// Run runs the app, opening docker/db/etc connections. This function blocks
// until an error occurs, or the app closes.
func (a *App) Run() error {
	if a.cli == nil {
		cli, err := client.NewEnvClient()
		if err != nil {
			return err
		}

		a.cli = cli
		a.cliClose = cli.Close
	}

	if a.cliClose != nil {
		defer func() {
			if err := a.cliClose(); err != nil {
				a.logger.Error("error closing docker client", zap.Error(err))
			}
		}()
	}

	// Sanity check Docker client
	_, err := a.cli.Info(context.Background())
	if err != nil {
		return err
	}

	if a.db, err = sql.Open("postgres", a.dbString); err != nil {
		return err
	}
	defer func() {
		if err := a.db.Close(); err != nil {
			a.logger.Error("error closing database connection", zap.Error(err))
		}
	}()

	if err := a.db.Ping(); err != nil {
		return err
	}

	a.specStore = models.NewSpecStore(a.db)
	a.instanceStore = models.NewInstanceStore(a.db)

	a.srv = &http.Server{
		Addr:    a.addr,
		Handler: a.router,
	}

	a.logger.Info("starting http server", zap.String("addr", a.addr))
	return a.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the bot. It behaves like
// http.Server.Shutdown.
func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("shutting down http server", zap.String("addr", a.addr))
	return a.srv.Shutdown(ctx)
}

// httpError writes a message to the writer. If the app is in debug mode,
// then the message will be written, otherwise the default message
// for the provided status code will be used.
func (a *App) httpError(w http.ResponseWriter, msg string, code int) {
	if a.debug {
		http.Error(w, msg, code)
	} else {
		http.Error(w, http.StatusText(code), code)
	}
}
