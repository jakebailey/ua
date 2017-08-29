package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/jakebailey/ua/app"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var args = struct {
	Debug      bool   `arg:"env:UA_DEBUG,help:enables pretty logging and extra debug routes"`
	StaticPath string `arg:"--static-path,env:UA_STATIC_PATH,help:path to static directory; if not provided embedded assets are used"`

	Addr              string `arg:"env:UA_ADDR,help:address to run the http server on"`
	Database          string `arg:"required,env:UA_DATABASE,help:postgres database connection string"`
	CertFile          string `arg:"--cert-file,env:UA_CERT_FILE,help:path to HTTPS certificate file"`
	KeyFile           string `arg:"--key-file,env:UA_KEY_FILE,help:path to HTTPS key file"`
	AESKey            string `arg:"--aes-key,required,env:UA_AES_KEY,help:base64 encoded AES key"`
	LetsEncryptDomain string `arg:"--letsencrypt-domain,env:UA_LE_DOMAIN,help:domain to run Let's Encrypt on"`
	AssignmentPath    string `arg:"--assignment-path,env:UA_ASSIGNMENT_PATH,help:path to assignments directory"`

	CleanInactiveEvery time.Duration `arg:"--clean-inactive-every,env:UA_CLEAN_INACTIVE_EVERY,help:how often to clean up inactive instances"`
	CheckExpiredEvery  time.Duration `arg:"--check-expired-every,env:UA_CHECK_EXPIRED_EVERY,help:how often to check for expired instances"`
	WebsocketTimeout   time.Duration `arg:"--websocket-timeout,env:UA_WS_TIMEOUT,help:maximum duration a websocket can be inactive"`
	InstanceExpire     time.Duration `arg:"--instance-expire,env:UA_INSTANCE_EXPIRE,help:duration to expire instances after"`
}{
	Addr:               app.DefaultAddr,
	AssignmentPath:     app.DefaultAssignmentPath,
	CleanInactiveEvery: app.DefaultCleanInactiveEvery,
	CheckExpiredEvery:  app.DefaultCheckExpiredEvery,
	WebsocketTimeout:   app.DefaultWebsocketTimeout,
	InstanceExpire:     app.DefaultInstanceExpire,
}

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "error loading .env file:", err)
		os.Exit(1)
	}

	p := arg.MustParse(&args)

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
		app.StaticPath(args.StaticPath),
		app.AESKey(key),
		app.AssignmentPath(args.AssignmentPath),
		app.CleanInactiveEvery(args.CleanInactiveEvery),
		app.CheckExpiredEvery(args.CheckExpiredEvery),
		app.WebsocketTimeout(args.WebsocketTimeout),
		app.InstanceExpire(args.InstanceExpire),
	}

	if args.LetsEncryptDomain != "" {
		if args.CertFile != "" || args.KeyFile != "" {
			p.Fail("cannot use both Let's Encrypt and regular TLS certs at the same time")
		}

		options = append(options, app.LetsEncryptDomain(args.LetsEncryptDomain))
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
