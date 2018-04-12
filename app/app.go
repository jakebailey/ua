package app

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"errors"
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
	"github.com/jakebailey/ua/pkg/docker/dcompat"
	"github.com/jakebailey/ua/pkg/expire"
	"github.com/jakebailey/ua/pkg/sched"
	cache "github.com/patrickmn/go-cache"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"
)

// App is the main application for uAssign.
type App struct {
	config Config

	router chi.Router
	srv    *http.Server

	logger *zap.Logger
	spew   *spew.ConfigState

	cli client.CommonAPIClient

	db            *sql.DB
	specStore     *models.SpecStore
	instanceStore *models.InstanceStore

	cleanInactiveRunner *sched.Runner
	checkExpiredRunner  *sched.Runner

	wsWG      sync.WaitGroup
	wsManager *expire.Manager

	aesKey []byte

	autoPullRunner *sched.Runner
	autoPullImages *cache.Cache

	pruneRunner *sched.Runner

	dockerCheckRunner   *sched.Runner
	dockerOk            atomic.Bool
	databaseCheckRunner *sched.Runner
	databaseOk          atomic.Bool
}

// NewApp creates a new app, with an optional list of options.
// This function does not open any connections, only setting up the app before
// Run is called.
func NewApp(config *Config, options ...Option) (*App, error) {
	a := &App{
		config: DefaultConfig,
		logger: zap.NewNop(),
		spew:   &spew.ConfigState{Indent: "    ", ContinueOnMethod: true},
	}

	if config != nil {
		a.config = *config
	}

	if err := a.config.Verify(); err != nil {
		return nil, err
	}

	if a.config.AESKey != "" {
		key, err := base64.StdEncoding.DecodeString(a.config.AESKey)
		if err != nil {
			return nil, err
		}
		a.aesKey = key
	}

	for _, o := range options {
		o(a)
	}

	switch len(a.aesKey) {
	case 0:
		return nil, errors.New("zero-length aes key")
	case 16, 24, 32:
		// Do nothing.
	default:
		return nil, errors.New("AES key must be of length 16, 24, or 32")
	}

	a.route()

	return a, nil
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
	a.cli = dcompat.Wrap(a.cli)

	var err error

	a.precheckDockerTask()

	if a.db, err = sql.Open("postgres", a.config.Database); err != nil {
		a.logger.Error("error opening database",
			zap.Error(err),
		)
		return err
	}

	a.precheckDatabaseTask()

	if a.config.MigrateReset {
		if err = migrations.Reset(a.db); err != nil {
			a.logger.Error("error resetting database",
				zap.Error(err),
			)
			return err
		}
	} else if a.config.MigrateUp {
		if err = migrations.Up(a.db); err != nil {
			a.logger.Error("error migrating database up",
				zap.Error(err),
			)
			return err
		}
	}

	a.specStore = models.NewSpecStore(a.db)
	a.instanceStore = models.NewInstanceStore(a.db)

	if a.config.ForceInactive {
		// Ensure that the database doesn't have any already active or uncleaned
		// instances. For now, only one of these servers will run at a time. This
		// will need to be removed once the platform improves.
		//
		// Update: This should really only be set locally, when the server
		// doesn't stick around long enough to manage Docker properly.
		a.markAllInstancesCleanedAndInactive()
	}

	// TODO: merge these scheduled tasks into some library that handles many.
	a.cleanInactiveRunner = sched.NewRunner(a.cleanInactiveInstances, a.config.CleanInactiveEvery)
	a.cleanInactiveRunner.Start()

	a.checkExpiredRunner = sched.NewRunner(a.checkExpiredInstances, a.config.CheckExpiredEvery)
	a.checkExpiredRunner.Start()

	a.autoPullImages = cache.New(a.config.AutoPullExpiry, time.Minute)

	if !a.config.DisableAutoPull {
		a.autoPullRunner = sched.NewRunner(a.autoPull, a.config.AutoPullEvery)
		a.autoPullRunner.Start()
	}

	a.pruneRunner = sched.NewRunner(a.pruneDocker, a.config.PruneEvery)
	a.pruneRunner.Start()

	a.wsManager = expire.NewManager(time.Minute, a.config.WebsocketTimeout)
	a.wsManager.Run()

	// TODO: Make this configurable.
	a.dockerCheckRunner = sched.NewRunner(a.precheckDockerTask, 30*time.Second)
	a.dockerCheckRunner.Start()

	// TODO: Make this configurable.
	a.databaseCheckRunner = sched.NewRunner(a.precheckDatabaseTask, 30*time.Second)
	a.databaseCheckRunner.Start()

	errorLog, err := zap.NewStdLogAt(a.logger, zap.DebugLevel)
	if err != nil {
		return err
	}

	a.srv = &http.Server{
		Handler:  a.router,
		ErrorLog: errorLog,
	}

	if a.config.LetsEncryptDomain != "" {
		// l := autocert.NewListener(a.config.LetsEncryptDomain)

		// Replacement for the above, since the tls-sni challenge has been disabled.
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(a.config.LetsEncryptDomain),
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

		a.logger.Info("starting http server", zap.String("domain", a.config.LetsEncryptDomain))

		a.srv.Addr = ":https"
		a.srv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}

		err = a.srv.ListenAndServeTLS("", "")

		// err = a.srv.Serve(l)
	} else {
		a.srv.Addr = a.config.Addr

		a.logger.Info("starting http server", zap.String("addr", a.config.Addr))

		if a.config.CertFile != "" && a.config.KeyFile != "" {
			err = a.srv.ListenAndServeTLS(a.config.CertFile, a.config.KeyFile)
		} else {
			if !a.config.Debug {
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
	a.dockerCheckRunner.Stop()
	a.databaseCheckRunner.Stop()

	a.logger.Info("expiring all websocket connections")
	a.wsManager.Stop()
	a.wsManager.ExpireAndRemoveAll()

	a.logger.Info("waiting for websocket handlers to exit")
	a.wsWG.Wait()

	a.cleanupLeftoverInstances()

	if a.config.ForceInactive {
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
