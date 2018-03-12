package app

import (
	"context"
	"crypto/tls"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/lib/pq" // postgresql driver

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
	"github.com/jakebailey/ua/migrations"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/pkg/expire"
	"github.com/jakebailey/ua/pkg/sched"
	cache "github.com/patrickmn/go-cache"
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

	// DefaultAutoPullEvery is the interval at which the server will attempt
	// to pull images that have been recently used (to keep them updated).
	DefaultAutoPullEvery = time.Hour

	// DefaultAutoPullExpiry defines what the autopuller defines as "recent".
	DefaultAutoPullExpiry = 30 * time.Minute

	// DefaultPruneEvery is the interval at which the server will prune docker.
	DefaultPruneEvery = time.Hour
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

	cli           client.CommonAPIClient
	disableLimits bool

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

	aesKey     []byte
	pprofToken string

	autoPullDisabled bool
	autoPullRunner   *sched.Runner
	autoPullImages   *cache.Cache
	autoPullEvery    time.Duration
	autoPullExpiry   time.Duration

	pruneRunner *sched.Runner
	pruneEvery  time.Duration

	forceInactive bool
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
		autoPullEvery:      DefaultAutoPullEvery,
		autoPullExpiry:     DefaultAutoPullExpiry,
		pruneEvery:         DefaultPruneEvery,
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
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			a.logger.Error("error creating docker env client",
				zap.Error(err),
			)
			return err
		}

		a.cli = cli
	}

	var err error

	a.initDocker()

	if a.db, err = sql.Open("postgres", a.dbString); err != nil {
		a.logger.Error("error opening database",
			zap.Error(err),
		)
		return err
	}

	if err = a.db.Ping(); err != nil {
		a.logger.Warn("error pinging database, continuing anyway",
			zap.Error(err),
		)
	}

	if a.migrateReset {
		if err = migrations.Reset(a.db); err != nil {
			a.logger.Error("error resetting database",
				zap.Error(err),
			)
			return err
		}
	} else if a.migrateUp {
		if err = migrations.Up(a.db); err != nil {
			a.logger.Error("error migrating database up",
				zap.Error(err),
			)
			return err
		}
	}

	a.specStore = models.NewSpecStore(a.db)
	a.instanceStore = models.NewInstanceStore(a.db)

	if a.forceInactive {
		// Ensure that the database doesn't have any already active or uncleaned
		// instances. For now, only one of these servers will run at a time. This
		// will need to be removed once the platform improves.
		//
		// Update: This should really only be set locally, when the server
		// doesn't stick around long enough to manage Docker properly.
		a.markAllInstancesCleanedAndInactive()
	}

	// TODO: merge these scheduled tasks into some library that handles many.
	a.cleanInactiveRunner = sched.NewRunner(a.cleanInactiveInstances, a.cleanInactiveEvery)
	a.cleanInactiveRunner.Start()

	a.checkExpiredRunner = sched.NewRunner(a.checkExpiredInstances, a.checkExpiredEvery)
	a.checkExpiredRunner.Start()

	a.autoPullImages = cache.New(a.autoPullExpiry, time.Minute)

	if !a.autoPullDisabled {
		a.autoPullRunner = sched.NewRunner(a.autoPull, a.autoPullEvery)
		a.autoPullRunner.Start()
	}

	a.pruneRunner = sched.NewRunner(a.pruneDocker, a.pruneEvery)
	a.pruneRunner.Start()

	a.wsManager = expire.NewManager(time.Minute, a.wsTimeout)
	a.wsManager.Run()

	errorLog, err := zap.NewStdLogAt(a.logger, zap.DebugLevel)
	if err != nil {
		return err
	}

	a.srv = &http.Server{
		Handler:  a.router,
		ErrorLog: errorLog,
	}

	if a.letsEncrypt {
		// l := autocert.NewListener(a.letsEncryptDomain)

		// Replacement for the above, since the tls-sni challenge has been disabled.
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(a.letsEncryptDomain),
		}

		cacheBase := "golang-autocert"
		var cacheDir string

		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			cacheDir = filepath.Join(xdg, cacheBase)
		} else {
			homeDir := os.Getenv("HOME")
			if homeDir == "" {
				homeDir = "/"
			}
			cacheDir = filepath.Join(homeDir, ".cache", cacheBase)
		}

		if err = os.MkdirAll(cacheDir, 0700); err != nil {
			a.logger.Warn("warning: autocert not using a cache",
				zap.Error(err),
			)
		} else {
			m.Cache = autocert.DirCache(cacheDir)
		}

		go func() {
			// tls-http-01 challenge
			if herr := http.ListenAndServe(":http", m.HTTPHandler(nil)); herr != nil {
				a.logger.Error("regular http redirect error",
					zap.Error(herr),
				)
			}
		}() // Never exits, unless the server has an error. TODO: fix this.

		a.logger.Info("starting http server", zap.String("domain", a.letsEncryptDomain))

		a.srv.Addr = ":https"
		a.srv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}

		err = a.srv.ListenAndServeTLS("", "")

		// err = a.srv.Serve(l)
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
	a.autoPullRunner.Stop()
	a.pruneRunner.Stop()

	a.logger.Info("expiring all websocket connections")
	a.wsManager.Stop()
	a.wsManager.ExpireAndRemoveAll()

	a.logger.Info("waiting for websocket handlers to exit")
	a.wsWG.Wait()

	a.cleanupLeftoverInstances()

	if a.forceInactive {
		// This shouldn't be needed, but by this point all instances should be both
		// cleaned and inactive, so just force everything into the correct state
		// ignoring all of the errors that happened during cleanup.
		//
		// Update: This should really only be set locally, when the server
		// doesn't stick around long enough to manage Docker properly.
		a.markAllInstancesCleanedAndInactive()
	}

	a.logger.Info("closing docker client")
	if err := a.cli.Close(); err != nil {
		a.logger.Error("error closing docker client", zap.Error(err))
	}

	a.logger.Info("closing database connection")
	if err := a.db.Close(); err != nil {
		a.logger.Error("error closing database connection", zap.Error(err))
	}
}

func (a *App) initDocker() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dockerPing, err := a.cli.Ping(ctx)
	if err != nil {
		a.logger.Warn("error pinging docker daemon, continuing anyway",
			zap.Error(err),
		)
	} else {
		a.cli.NegotiateAPIVersionPing(dockerPing)
	}

	a.logger.Info("negotiated Docker API version",
		zap.String("version", a.cli.ClientVersion()),
	)
}
