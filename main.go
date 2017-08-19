package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/app"
	"github.com/rs/zerolog"
)

var args = struct {
	Addr  string `arg:"env"`
	Debug bool   `arg:"env"`
}{
	Addr: ":8000",
}

func main() {
	arg.MustParse(&args)

	spew.Config.Indent = "    "
	spew.Config.ContinueOnMethod = true

	var log zerolog.Logger

	if args.Debug {
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
			Timestamp().
			Logger()

	} else {
		log = zerolog.New(os.Stderr).With().
			Timestamp().
			Logger()
	}

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatal().Err(err).Msg("error creating docker client")
	}

	defer func() {
		if err := cli.Close(); err != nil {
			log.Error().Err(err).Msg("error closing docker client")
		}
	}()

	options := []app.Option{app.Logger(log)}

	if args.Debug {
		options = append(options, app.Debug())
	}

	a := app.NewApp(cli, options...)

	addr := args.Addr
	log = log.With().Str("addr", addr).Logger()

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
