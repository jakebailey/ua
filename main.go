package main

import (
	"encoding/base64"
	"os"
	"os/signal"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/jakebailey/ua/app"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var args = struct {
	Debug bool `arg:"env,help:enables pretty logging and extra debug routes"`

	Addr     string `arg:"env,help:address to run the http server on"`
	Database string `arg:"required,env,help:postgres database connection string"`
	CertFile string `arg:"env,help:path to HTTPS certificate file"`
	KeyFile  string `arg:"env,help:path to HTTPS key file"`
	AESKey   string `arg:"required,env,help:base64 encoded AES key"`

	AssignmentPath     string        `arg:"--assignment-path,env,help:path to assignments directory"`
	CleanInactiveEvery time.Duration `arg:"--clean-inactive-every,help:how often to clean up inactive instances"`
	CheckExpiredEvery  time.Duration `arg:"--check-expired-every,help:how often to check for expired instances"`
	WebsocketTimeout   time.Duration `arg:"--websocket-timeout,help:maximum duration a websocket can be inactive"`
	InstanceExpire     time.Duration `arg:"--instance-expire,help:duration to expire instances after"`
}{
	Addr:               app.DefaultAddr,
	AssignmentPath:     app.DefaultAssignmentPath,
	CleanInactiveEvery: app.DefaultCleanInactiveEvery,
	CheckExpiredEvery:  app.DefaultCheckExpiredEvery,
	WebsocketTimeout:   app.DefaultWebsocketTimeout,
	InstanceExpire:     app.DefaultInstanceExpire,
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

	key, err := base64.StdEncoding.DecodeString(args.AESKey)
	if err != nil {
		logger.Fatal("error decoding AES key",
			zap.Error(err),
		)
	}

	options := []app.Option{
		app.Logger(logger),
		app.Addr(args.Addr),
		app.Debug(args.Debug),
		app.AESKey(key),
		app.AssignmentPath(args.AssignmentPath),
		app.CleanInactiveEvery(args.CleanInactiveEvery),
		app.CheckExpiredEvery(args.CheckExpiredEvery),
		app.WebsocketTimeout(args.WebsocketTimeout),
		app.InstanceExpire(args.InstanceExpire),
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
