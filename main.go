package main

import (
	"os"
	"os/signal"

	"github.com/alexflint/go-arg"
	"github.com/jakebailey/ua/app"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var args = struct {
	Addr     string `arg:"env,help:address to run the http server on"`
	Debug    bool   `arg:"env,help:enables pretty logging and extra debug routes"`
	Database string `arg:"required,env,help:postgres database connection string"`
	CertFile string `arg:"env,help:path to HTTPS certificate file"`
	KeyFile  string `arg:"env,help:path to HTTPS key file"`
}{
	Addr: ":8000",
}

func main() {
	arg.MustParse(&args)

	var logConfig zap.Config

	if args.Debug {
		logConfig = zap.NewDevelopmentConfig()
		logConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		logConfig = zap.NewProductionConfig()
	}

	logger, err := logConfig.Build()
	if err != nil {
		panic(err)
	}

	options := []app.Option{
		app.Logger(logger),
		app.Addr(args.Addr),
		app.Debug(args.Debug),
	}

	if args.CertFile != "" || args.KeyFile != "" {
		options = append(options, app.TLS(args.CertFile, args.KeyFile))
	}

	a := app.NewApp(args.Database, options...)

	go func() {
		logger.Info("starting app")
		if err := a.Run(); err != nil {
			logger.Fatal("app.Run error",
				zap.Error(err),
			)
		}
	}()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	<-stopChan
	logger.Info("shutting down app")
	a.Shutdown()
}
