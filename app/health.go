package app

import (
	"context"

	"github.com/etherlabsio/healthcheck"
	"github.com/go-chi/chi"
)

func (a *App) routeHealth(r chi.Router) {
	dockerCheck := func(ctx context.Context) error {
		_, err := a.cli.Ping(ctx)
		return err
	}

	databaseCheck := func(ctx context.Context) error {
		return a.db.PingContext(ctx)
	}

	r.Mount("/", healthcheck.Handler(
		healthcheck.WithChecker("docker", healthcheck.CheckerFunc(dockerCheck)),
		healthcheck.WithChecker("database", healthcheck.CheckerFunc(databaseCheck)),
	))
}
