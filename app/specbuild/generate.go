package specbuild

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/js"
	"go.uber.org/zap"
)

// ErrNoJS is returned by Generate when no JS code was found, meaning that the
// legacy code should be run instead.
var ErrNoJS = errors.New("specbuild: no JS code found")

// GenerateOutput is the output object given by the assignment's generate
// function.
type GenerateOutput struct {
	ImageName  string
	Dockerfile string
	Init       *bool

	PostBuild []Action

	User       string
	Cmd        []string
	Env        []string
	WorkingDir string
}

// Generate attempts to run the generate function of the assignment's module
// and returns its output.
func Generate(ctx context.Context, assignmentPath string, specData interface{}) (*GenerateOutput, error) {
	logger := ctxlog.FromContext(ctx)

	jsPath := filepath.Join(assignmentPath, "index.js")
	if _, err := os.Stat(jsPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoJS
		}

		logger.Error("error trying to load JS",
			zap.String("filename", jsPath),
			zap.Error(err),
		)

		return nil, err
	}

	consoleOutput := &bytes.Buffer{}
	runtime := js.NewRuntime(&js.Options{
		Stdout:       consoleOutput,
		ModuleLoader: js.PathsModuleLoader(assignmentPath),
		FileReader:   js.PathsFileReader(assignmentPath),
	})
	defer runtime.Destroy()

	var out GenerateOutput

	runtime.Set("__specData__", specData)
	if err := runtime.Run(ctx, "require('index.js').generate(__specData__);", &out); err != nil {
		logger.Error("javascript error",
			zap.Error(err),
			zap.String("console", consoleOutput.String()),
		)
		return nil, err
	}

	return &out, nil
}
