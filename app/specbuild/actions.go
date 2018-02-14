package specbuild

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/app/gobuild"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/dexec"
	"go.uber.org/zap"
)

// Action defines an action that can be performed on a running container.
type Action struct {
	Action     string
	User       string
	WorkingDir string

	// Exec action
	Cmd   []string
	Env   []string
	Stdin *string

	// Write/append option
	Contents       string
	ContentsBase64 bool
	Filename       string

	// Gobuild action
	SrcPath  string
	Packages []string
	LDFlags  string
}

var actionFuncs = map[string]func(ctx context.Context, cli client.CommonAPIClient, containerID string, ac Action) error{
	"exec":    actionExec,
	"write":   actionWriteAppend,
	"append":  actionWriteAppend,
	"gobuild": actionGobuild,
}

// PerformActions performs the given actions on the specified container.
func PerformActions(ctx context.Context, cli client.CommonAPIClient, containerID string, actions []Action) error {
	logger := ctxlog.FromContext(ctx)

	for _, ac := range actions {
		fn, ok := actionFuncs[ac.Action]
		if !ok {
			logger.Warn("unknown action",
				zap.String("action", ac.Action),
			)
			continue
		}

		if err := fn(ctx, cli, containerID, ac); err != nil {
			return err
		}
	}

	return nil
}

func actionExec(ctx context.Context, cli client.CommonAPIClient, containerID string, ac Action) error {
	logger := ctxlog.FromContext(ctx)

	logger.Debug("exec action",
		zap.String("user", ac.User),
		zap.Strings("cmd", ac.Cmd),
		zap.Strings("env", ac.Env),
		zap.String("working_dir", ac.WorkingDir),
	)

	ec := dexec.Config{
		User:       ac.User,
		Cmd:        ac.Cmd,
		Env:        ac.Env,
		WorkingDir: ac.WorkingDir,
	}

	if ac.Stdin != nil {
		ec.Stdin = strings.NewReader(*ac.Stdin)
	}

	return dexec.Exec(ctx, cli, containerID, ec)
}

func actionWriteAppend(ctx context.Context, cli client.CommonAPIClient, containerID string, ac Action) error {
	logger := ctxlog.FromContext(ctx)

	logger.Debug(ac.Action+" action",
		zap.String("user", ac.User),
		zap.String("filename", ac.Filename),
		zap.String("working_dir", ac.WorkingDir),
		zap.Bool("contents_base64", ac.ContentsBase64),
	)

	var r io.Reader = strings.NewReader(ac.Contents)
	if ac.ContentsBase64 {
		r = base64.NewDecoder(base64.StdEncoding, r)
	}

	redir := ">"

	if ac.Action == "append" {
		redir = ">>"
	}

	ec := dexec.Config{
		User:       ac.User,
		Cmd:        []string{"sh", "-c", "cat " + redir + " " + ac.Filename},
		WorkingDir: ac.WorkingDir,
		Stdin:      r,
	}

	return dexec.Exec(ctx, cli, containerID, ec)
}

func actionGobuild(ctx context.Context, cli client.CommonAPIClient, containerID string, ac Action) error {
	logger := ctxlog.FromContext(ctx)

	logger.Debug("gobuild action",
		zap.String("src_path", ac.SrcPath),
		zap.Strings("packages", ac.Packages),
		zap.String("ldflags", ac.LDFlags),
	)

	options := gobuild.Options{
		SrcPath:  ac.SrcPath,
		Packages: ac.Packages,
		LDFlags:  ac.LDFlags,
	}

	r, err := gobuild.Build(ctx, cli, options)
	if err != nil {
		return err
	}

	ec := dexec.Config{
		User:       "root",
		Cmd:        []string{"tar", "-x"},
		WorkingDir: "/bin",
		Stdin:      r,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}

	return dexec.Exec(ctx, cli, containerID, ec)
}