package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/app"
	"github.com/rs/zerolog"
)

func main() {
	spew.Config.Indent = "    "
	spew.Config.ContinueOnMethod = true

	addr := ":8000"

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
		Timestamp().
		Str("addr", addr).
		Logger()

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatal().Err(err).Msg("error creating docker client")
	}

	defer func() {
		if err := cli.Close(); err != nil {
			log.Error().Err(err).Msg("error closing docker client")
		}
	}()

	a := app.NewApp(cli, app.Debug(), app.Logger(log))

	srv := &http.Server{
		Addr:    addr,
		Handler: a,
	}

	go func() {
		log.Info().Msg("starting")
		if err := srv.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("ListenAndServe error")
		}
	}()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	<-stopChan
	log.Info().Msg("shutting down")

	ctx, canc := context.WithTimeout(context.Background(), 5*time.Second)
	defer canc()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("error shutting down")
	}
}
