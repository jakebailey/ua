package app

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func (a *App) precheckDocker() bool {
	return a.dockerOk.Load()
}

func (a *App) precheckDockerMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if a.precheckDocker() {
			next.ServeHTTP(w, r)
			return
		}

		a.httpError(w, "docker precheck failed", http.StatusInternalServerError)
	}
	return http.HandlerFunc(fn)
}

func (a *App) precheckDockerTask() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prev := a.dockerOk.Load()

	dockerPing, err := a.cli.Ping(ctx)

	if err == nil {
		a.dockerOk.Store(true)

		if !prev {
			a.cli.NegotiateAPIVersionPing(dockerPing)
			a.logger.Info("negotiated Docker API version",
				zap.String("version", a.cli.ClientVersion()),
			)
		}

		return
	}

	a.dockerOk.Store(false)
	a.logger.Warn("error pinging docker daemon, setting dockerOk to false",
		zap.Error(err),
	)
}

func (a *App) precheckDatabase() bool {
	return a.databaseOk.Load()
}

func (a *App) precheckDatabaseMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if a.precheckDatabase() {
			next.ServeHTTP(w, r)
			return
		}

		a.httpError(w, "database precheck failed", http.StatusInternalServerError)
	}
	return http.HandlerFunc(fn)
}

func (a *App) precheckDatabaseTask() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prev := a.databaseOk.Load()

	err := a.db.PingContext(ctx)

	if err == nil {
		a.databaseOk.Store(true)

		if !prev {
			a.logger.Info("connected to database")
		}

		return
	}

	a.dockerOk.Store(false)
	a.logger.Warn("error pinging database, setting databaseOk to false",
		zap.Error(err),
	)
}
