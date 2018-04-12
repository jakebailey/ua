package main

import (
	"log"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/journal"
	"github.com/jakebailey/ua/app"
	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Copy of app.Config, but with struct tags. This means that a value of this
// type can be converted directly to app.Config and passed in, rather than
// copying each field one by one, and for app.DefaultConfig to be used as
// the defaults rather than repeating a bunch of values.
type flaggedConfig struct {
	Addr string `long:"addr" env:"UA_ADDR" description:"Address to run the server on"`

	LetsEncryptDomain string `long:"letsencrypt-domain" env:"UA_LE_DOMAIN" description:"Domain to run Let's Encrypt on"`
	CertFile          string `long:"cert-file" env:"UA_CERT_FILE" description:"Path to HTTPS certificate file"`
	KeyFile           string `long:"key-file" env:"UA_KEY_FILE" description:"Path to HTTPS key file"`

	Database     string `long:"database" required:"true" env:"UA_DATABASE" description:"PostgreSQL database connection string"`
	MigrateUp    bool   `long:"migrate-up" env:"UA_MIGRATE_UP" description:"Run migrations up after database connection"`
	MigrateReset bool   `long:"migrate-reset" env:"UA_MIGRATE_RESET" description:"Reset database and run migrations up after database connection"`

	AssignmentPath string `long:"assignment-path" env:"UA_ASSIGNMENT_PATH" description:"Path to assignments directory"`
	StaticPath     string `long:"static-path" env:"UA_STATIC_PATH" description:"Path to static directory; if not provided embedded assets are used"`

	AESKey string `long:"aes-key" required:"true" env:"UA_AES_KEY" description:"base64 encoded AES key"`

	CleanInactiveEvery time.Duration `long:"clean-inactive-every" env:"UA_CLEAN_INACTIVE_EVERY" description:"How often to clean up inactive instances"`
	CheckExpiredEvery  time.Duration `long:"check-expired-every" env:"UA_CHECK_EXPIRED_EVERY" description:"How often to check for expired instances"`
	WebsocketTimeout   time.Duration `long:"websocket-timeout" env:"UA_WS_TIMEOUT" description:"Maximum duration a websocket can be inactive"`
	InstanceExpire     time.Duration `long:"instance-expire" env:"UA_INSTANCE_EXPIRE" description:"Duration to expire instances after"`
	ForceInactive      bool          `long:"force-inactive" env:"UA_FORCE_INACTIVE" description:"Force all instances to be inactive on startup/shutdown"`

	DisableLimits bool `long:"disable-limits" env:"UA_DISABLE_LIMITS" description:"Disable container limits"`

	DisableAutoPull bool          `long:"disable-auto-pull" env:"UA_AUTO_PULL" description:"Disable image autopull"`
	AutoPullEvery   time.Duration `long:"auto-pull-every" env:"UA_AUTO_PULL_EVERY" description:"How often to auto-pull recently used images"`
	AutoPullExpiry  time.Duration `long:"auto-pull-expiry" env:"UA_AUTO_PULL_EXPIRY" description:"How often an image must be used to be autopulled"`

	PruneEvery time.Duration `long:"prune-every" env:"UA_PRUNE_EVERY" description:"How often to prune Docker"`

	// TODO: Split this out into DebugRoutes and something like DebugLogging.
	Debug      bool   `long:"debug" env:"UA_DEBUG" description:"Enables pretty logging and extra debug routes"`
	PProfToken string `long:"pprof-token" env:"UA_PPROF_TOKEN" description:"Token/password for pprof debug endpoint (disabled if not set unless in debug mode)"`
}

var args = struct {
	AppConfig flaggedConfig

	JournaldDirect bool `long:"journald-direct" env:"UA_JOURNALD_DIRECT" description:"Send logs directly to systemd to prevent line truncation"`
}{
	AppConfig: flaggedConfig(app.DefaultConfig),
}

func main() {
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
	}

	if _, err := flags.Parse(&args); err != nil {
		// Default flag parser prints messages, so just exit.
		os.Exit(1)
	}

	var logConfig zap.Config

	if args.AppConfig.Debug {
		logConfig = zap.NewDevelopmentConfig()
		logConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		logConfig = zap.NewProductionConfig()
	}

	var logger *zap.Logger

	if !args.JournaldDirect {
		var err error
		logger, err = logConfig.Build()
		if err != nil {
			panic(err)
		}
	} else {
		logger = buildJournaldLogger(logConfig)
	}

	undoStdlog := zap.RedirectStdLog(logger)
	defer undoStdlog()

	appConfig := app.Config(args.AppConfig)
	a, err := app.NewApp(&appConfig, app.WithLogger(logger))
	if err != nil {
		log.Fatalln(err)
		return
	}

	go func() {
		stopChan := make(chan os.Signal, 1)
		signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

		<-stopChan
		logger.Info("shutting down app")
		a.Shutdown()
	}()

	logger.Info("starting app")
	if err := a.Run(); err != nil {
		logger.Fatal("app.Run error",
			zap.Error(err),
		)
	}
}

type journaldLogger struct{}

func (j journaldLogger) Write(p []byte) (n int, err error) {
	if err := journal.Send(string(p), journal.PriInfo, nil); err != nil {
		return 0, err
	}
	return len(p), nil
}

func buildJournaldLogger(cfg zap.Config) *zap.Logger {
	enc := zapcore.NewJSONEncoder(cfg.EncoderConfig)
	ws := zapcore.Lock(zapcore.AddSync(journaldLogger{}))

	opts := []zap.Option{zap.ErrorOutput(ws)}

	if cfg.Development {
		opts = append(opts, zap.Development())
	}

	if !cfg.DisableCaller {
		opts = append(opts, zap.AddCaller())
	}

	stackLevel := zap.ErrorLevel
	if cfg.Development {
		stackLevel = zap.WarnLevel
	}
	if !cfg.DisableStacktrace {
		opts = append(opts, zap.AddStacktrace(stackLevel))
	}

	if cfg.Sampling != nil {
		opts = append(opts, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSampler(core, time.Second, cfg.Sampling.Initial, cfg.Sampling.Thereafter)
		}))
	}

	if len(cfg.InitialFields) > 0 {
		fs := make([]zapcore.Field, 0, len(cfg.InitialFields))
		keys := make([]string, 0, len(cfg.InitialFields))
		for k := range cfg.InitialFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fs = append(fs, zap.Any(k, cfg.InitialFields[k]))
		}
		opts = append(opts, zap.Fields(fs...))
	}

	logger := zap.New(
		zapcore.NewCore(enc, ws, cfg.Level),
		opts...,
	)

	return logger
}
