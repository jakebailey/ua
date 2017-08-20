package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/jakebailey/ua/app"
	"github.com/rs/zerolog"
)

var args = struct {
	Addr     string `arg:"env,help:address to run the http server on"`
	Debug    bool   `arg:"env,help:enables pretty logging and extra debug routes"`
	Database string `arg:"required,env,help:postgres database connection string"`
}{
	Addr: ":8000",
}

func main() {
	arg.MustParse(&args)

	var out io.Writer = os.Stderr
	if args.Debug {
		out = zerolog.ConsoleWriter{Out: os.Stderr}
	}

	log := zerolog.New(out).With().Timestamp().Logger()

	options := []app.Option{
		app.Logger(log),
		app.Addr(args.Addr),
		app.Debug(args.Debug),
	}

	a := app.NewApp(args.Database, options...)

	go func() {
		log.Info().Msg("starting app")
		if err := a.Run(); err != nil {
			log.Fatal().Err(err).Msg("app.Run error")
		}
	}()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	<-stopChan
	log.Info().Msg("shutting down app")

	ctx, canc := context.WithTimeout(context.Background(), 5*time.Second)
	defer canc()

	if err := a.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("error shutting down")
	}
}
