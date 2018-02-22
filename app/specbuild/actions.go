package specbuild

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/client"
	"github.com/hashicorp/go-gatedio"
	"github.com/jakebailey/ua/app/gobuild"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/dexec"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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

	// Parallel action
	Subactions []Action
}

type actionFunc func(ctx context.Context, cli client.CommonAPIClient, containerID string, ac Action) error

var actionFuncs = map[string]actionFunc{}

func init() {
	actionFuncs["exec"] = actionExec
	actionFuncs["write"] = actionWriteAppend
	actionFuncs["append"] = actionWriteAppend
	actionFuncs["gobuild"] = actionGobuild
	actionFuncs["parallel"] = actionParallel
}

func performAction(ctx context.Context, cli client.CommonAPIClient, containerID string, ac Action) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fn, ok := actionFuncs[ac.Action]
	if !ok {
		return fmt.Errorf("specbuild: unknown action %v", ac.Action)
	}
	return fn(ctx, cli, containerID, ac)
}

// PerformActions performs the given actions on the specified container.
func PerformActions(ctx context.Context, cli client.CommonAPIClient, containerID string, actions []Action) error {
	for _, ac := range actions {
		if err := performAction(ctx, cli, containerID, ac); err != nil {
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

	stdout := gatedio.NewByteBuffer()
	stderr := gatedio.NewByteBuffer()

	ec := dexec.Config{
		User:       ac.User,
		Cmd:        ac.Cmd,
		Env:        ac.Env,
		WorkingDir: ac.WorkingDir,
		Stdout:     stdout,
		Stderr:     stderr,
	}

	if ac.Stdin != nil {
		ec.Stdin = strings.NewReader(*ac.Stdin)
	}

	// TODO: Properly return stdout/stderr in the error value.
	if err := dexec.Exec(ctx, cli, containerID, ec); err != nil {
		logger.Warn("actionExec error",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
		)
		return err
	}

	return nil
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

	stdout := gatedio.NewByteBuffer()
	stderr := gatedio.NewByteBuffer()

	ec := dexec.Config{
		User:       ac.User,
		Cmd:        []string{"sh", "-c", "exec cat " + redir + " " + ac.Filename},
		WorkingDir: ac.WorkingDir,
		Stdin:      r,
		Stdout:     stdout,
		Stderr:     stderr,
	}

	// TODO: Properly return stdout/stderr in the error value.
	if err := dexec.Exec(ctx, cli, containerID, ec); err != nil {
		logger.Warn("actionWriteAppend error",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
		)
		return err
	}

	return nil
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

	stdout := gatedio.NewByteBuffer()
	stderr := gatedio.NewByteBuffer()

	ec := dexec.Config{
		User:       "root",
		Cmd:        []string{"tar", "-x"},
		WorkingDir: "/bin",
		Stdin:      r,
		Stdout:     stdout,
		Stderr:     stderr,
	}

	// TODO: Properly return stdout/stderr in the error value.
	if err := dexec.Exec(ctx, cli, containerID, ec); err != nil {
		logger.Warn("actionGobuild error",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
		)
		return err
	}

	return nil
}

func actionParallel(ctx context.Context, cli client.CommonAPIClient, containerID string, ac Action) error {
	logger := ctxlog.FromContext(ctx)

	logger.Debug("parallel action",
		zap.Int("subactions", len(ac.Subactions)),
	)

	g, ctx := errgroup.WithContext(ctx)

	for _, ac := range ac.Subactions {
		ac := ac

		g.Go(func() error {
			return performAction(ctx, cli, containerID, ac)
		})
	}

	return g.Wait()
}
