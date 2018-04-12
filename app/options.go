package app

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

// Option is a function that runs on an App to configure it.
type Option func(*App)

// WithLogger sets the logger used within the app. If not provided,
// DefaultLogger is used.
func WithLogger(logger *zap.Logger) Option {
	return func(a *App) {
		a.logger = logger
	}
}

// WithSpewConfig sets the spew config state used for various debugging
// endpoints in the app. If not provided, DefaultSpew is used.
func WithSpewConfig(c *spew.ConfigState) Option {
	if c == nil {
		panic("app: spew ConfigState cannot be nil")
	}

	return func(a *App) {
		a.spew = c
	}
}

// WithDockerClient sets the docker client used in the app.
func WithDockerClient(cli client.CommonAPIClient) Option {
	return func(a *App) {
		a.cli = cli
	}
}
