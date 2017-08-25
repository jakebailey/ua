package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/jakebailey/ua/app"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var args = struct {
	Addr       string `arg:"env,help:address to run the http server on"`
	Debug      bool   `arg:"env,help:enables pretty logging and extra debug routes"`
	Database   string `arg:"required,env,help:postgres database connection string"`
	APIKeyPath string `arg:"--api-key-path,env:API_KEY_PATH,help:path to api key csv"`
}{
	Addr: ":8000",
}

func main() {
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

	options := []app.Option{
		app.Logger(logger),
		app.Addr(args.Addr),
		app.Debug(args.Debug),
	}

	if !args.Debug {
		if args.APIKeyPath == "" {
			p.Fail("API key path must be provided if not in debug mode")
		}
	}

	if args.APIKeyPath != "" {
		keys, err := parseAPIKeys(args.APIKeyPath)
		if err != nil {
			log.Fatal("error parsing API keys",
				zap.Error(err),
				zap.String("path", args.APIKeyPath),
			)
		}
		options = append(options, app.APIKeys(keys))
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

func parseAPIKeys(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)

	contents, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	m := make(map[string]string)

	for i, line := range contents {
		if len(line) != 2 {
			return nil, fmt.Errorf("row %d: expected 2 columns, got %d", i, len(line))
		}

		key := line[0]
		name := line[1]

		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("row %d: API key cannot be empty", i)
		}

		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("row %d: name cannot be empty", i)
		}

		m[key] = name
	}

	return m, nil
}
