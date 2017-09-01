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
	"github.com/jakebailey/ua/migrations"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/sched"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"
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

	// DefaultInstanceExpire is the maximum duration an instance will be kept
	// on the server until it expires and a new instance must be created.
	DefaultInstanceExpire = 4 * time.Hour
)

// App is the main application for uAssign.
type App struct {
	debug bool

	addr           string
	assignmentPath string
	staticPath     string
	instanceExpire time.Duration

	letsEncrypt       bool
	letsEncryptDomain string

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
	migrateUp     bool
	migrateReset  bool

	cleanInactiveRunner *sched.Runner
	cleanInactiveEvery  time.Duration
	checkExpiredRunner  *sched.Runner
	checkExpiredEvery   time.Duration

	wsWG      sync.WaitGroup
	wsTimeout time.Duration
	wsManager *expire.Manager

	aesKey []byte
}

// NewApp creates a new app, with an optional list of options.
// This function does not open any connections, only setting up the app before
// Run is called.
func NewApp(dbString string, options ...Option) *App {
	a := &App{
		addr:               DefaultAddr,
		assignmentPath:     DefaultAssignmentPath,
		instanceExpire:     DefaultInstanceExpire,
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

// Run runs the app, opening docker/db/etc connections. This function blocks
// until an error occurs, or the app closes.
func (a *App) Run() error {
	if a.cli == nil {
		cli, err := client.NewEnvClient()
		if err != nil {
			a.logger.Error("error creating docker env client",
				zap.Error(err),
			)
			return err
		}

		a.cli = cli
		a.cliClose = cli.Close
	}

	var err error

	if dockerPing, err := a.cli.Ping(context.Background()); err != nil {
		a.logger.Warn("error pinging docker daemon, continuing anyway",
			zap.Error(err),
		)
	} else {
		a.cli.NegotiateAPIVersionPing(dockerPing)
	}

	if a.db, err = sql.Open("postgres", a.dbString); err != nil {
		a.logger.Error("error opening database",
			zap.Error(err),
		)
		return err
	}

	if err := a.db.Ping(); err != nil {
		a.logger.Warn("error pinging database, continuing anyway",
			zap.Error(err),
		)
	}

	if a.migrateReset {
		if err := migrations.Reset(a.db); err != nil {
			a.logger.Error("error resetting database",
				zap.Error(err),
			)
			return err
		}
	} else if a.migrateUp {
		if err := migrations.Up(a.db); err != nil {
			a.logger.Error("error migrating database up",
				zap.Error(err),
			)
			return err
		}
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
		Handler: a.router,
	}

	if a.letsEncrypt {
		l := autocert.NewListener(a.letsEncryptDomain)

		a.logger.Info("starting http server", zap.String("domain", a.letsEncryptDomain))

		err = a.srv.Serve(l)
	} else {
		a.srv.Addr = a.addr

		a.logger.Info("starting http server", zap.String("addr", a.addr))

		if a.tls {
			err = a.srv.ListenAndServeTLS(a.tlsCertFile, a.tlsKeyFile)
		} else {
			if !a.debug {
				a.logger.Warn("server running without https in production")
			}
			err = a.srv.ListenAndServe()
		}
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

	a.logger.Info("shutting down http server")
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
