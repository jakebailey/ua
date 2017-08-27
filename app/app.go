package app

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	_ "github.com/lib/pq" // postgresql driver

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
	"github.com/jakebailey/ua/expire"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/sched"
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

	// DefaultCleanInactiveEvery is the period at which the app will clean up
	// inactive images and containers.
	DefaultCleanInactiveEvery = time.Hour

	// DefaultCheckExpiredEvery is the period at which the app will check for
	// active instances past their expiry time and stop them.
	DefaultCheckExpiredEvery = time.Minute

	// DefaultWebsocketTimeout is the maximum duration a websocket can be
	// inactive before expiring.
	DefaultWebsocketTimeout = time.Hour
)

// App is the main application for uAssign.
type App struct {
	debug bool

	addr           string
	assignmentPath string
	staticPath     string

	tls                     bool
	tlsCertFile, tlsKeyFile string

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

	cleanInactiveRunner *sched.Runner
	cleanInactiveEvery  time.Duration
	checkExpiredRunner  *sched.Runner
	checkExpiredEvery   time.Duration

	wsWG      sync.WaitGroup
	wsTimeout time.Duration
	wsManager *expire.Manager
}

// NewApp creates a new app, with an optional list of options.
// This function does not open any connections, only setting up the app before
// Run is called.
func NewApp(dbString string, options ...Option) *App {
	a := &App{
		addr:               DefaultAddr,
		assignmentPath:     DefaultAssignmentPath,
		logger:             DefaultLogger,
		staticPath:         DefaultStaticPath,
		spew:               DefaultSpew,
		dbString:           dbString,
		cleanInactiveEvery: DefaultCleanInactiveEvery,
		checkExpiredEvery:  DefaultCheckExpiredEvery,
		wsTimeout:          DefaultWebsocketTimeout,
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

// TLS enables TLS for the app's HTTP server.
func TLS(certFile, certKey string) Option {
	return func(a *App) {
		a.tls = true
		a.tlsCertFile = certFile
		a.tlsKeyFile = certKey
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

	// Sanity check Docker client
	_, err := a.cli.Info(context.Background())
	if err != nil {
		return err
	}

	if a.db, err = sql.Open("postgres", a.dbString); err != nil {
		return err
	}

	if err := a.db.Ping(); err != nil {
		return err
	}

	a.specStore = models.NewSpecStore(a.db)
	a.instanceStore = models.NewInstanceStore(a.db)

	// Ensure that the database doesn't have any already active or uncleaned
	// instances. For now, only one of these servers will run at a time. This
	// will need to be removed once the platform improves.
	a.markAllInstancesCleanedAndInactive()

	a.cleanInactiveRunner = sched.NewRunner(a.cleanInactiveInstances, a.cleanInactiveEvery)
	a.cleanInactiveRunner.Start()

	a.checkExpiredRunner = sched.NewRunner(a.checkExpiredInstances, a.checkExpiredEvery)
	a.checkExpiredRunner.Start()

	a.wsManager = expire.NewManager(time.Minute, a.wsTimeout)
	a.wsManager.Run()

	a.srv = &http.Server{
		Addr:    a.addr,
		Handler: a.router,
	}

	a.logger.Info("starting http server", zap.String("addr", a.addr))

	if a.tls {
		err = a.srv.ListenAndServeTLS(a.tlsCertFile, a.tlsKeyFile)
	} else {
		if !a.debug {
			a.logger.Warn("server running without https in production")
		}
		err = a.srv.ListenAndServe()
	}

	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully shuts down the bot.
func (a *App) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a.logger.Info("shutting down http server", zap.String("addr", a.addr))
	if err := a.srv.Shutdown(ctx); err != nil {
		a.logger.Error("error shutting down http server",
			zap.Error(err),
		)
	}

	a.logger.Info("stopping scheduled tasks")
	a.cleanInactiveRunner.Stop()
	a.checkExpiredRunner.Stop()

	a.logger.Info("expiring all websocket connections")
	a.wsManager.Stop()
	a.wsManager.ExpireAndRemoveAll()

	a.logger.Info("waiting for websocket handlers to exit")
	a.wsWG.Wait()

	a.cleanupLeftoverInstances()

	// This shouldn't be needed, but by this point all instances should be both
	// cleaned and inactive, so just force everything into the correct state
	// ignoring all of the errors that happened during cleanup.
	a.markAllInstancesCleanedAndInactive()

	if a.cliClose != nil {
		a.logger.Info("closing docker client")
		if err := a.cliClose(); err != nil {
			a.logger.Error("error closing docker client", zap.Error(err))
		}
	}

	a.logger.Info("closing database connection")
	if err := a.db.Close(); err != nil {
		a.logger.Error("error closing database connection", zap.Error(err))
	}
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
