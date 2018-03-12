package dexec

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/dcompat"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Config configures an execution.
type Config struct {
	User       string
	Cmd        []string
	Env        []string
	WorkingDir string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Tty        bool
}

// ExitCodeError is an error which represents a non-zero exit code.
type ExitCodeError int

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("error code %d", e)
}

// Exec runs a process on a container, managing I/O, exit codes, etc.
//
// Exec will wait for the program to exit before returning. Cancel the context
// via a timeout/deadline/cancel to cause Exec to stop waiting and return.
func Exec(ctx context.Context, cli client.CommonAPIClient, containerID string, config Config) error {
	logger := ctxlog.FromContext(ctx)

	execConfig := types.ExecConfig{
		User:         config.User,
		Cmd:          config.Cmd,
		Env:          config.Env,
		WorkingDir:   config.WorkingDir,
		AttachStdin:  config.Stdin != nil,
		AttachStdout: config.Stdout != nil,
		AttachStderr: config.Stderr != nil,
		Tty:          config.Tty,
	}
	execConfig = dcompat.ExecConfig(cli, execConfig)

	logger.Debug("dexec",
		zap.String("user", execConfig.User),
		zap.Strings("cmd", execConfig.Cmd),
		zap.Strings("env", execConfig.Env),
		zap.String("working_dir", execConfig.WorkingDir),
		zap.Bool("stdin", execConfig.AttachStdin),
		zap.Bool("stdout", execConfig.AttachStdout),
		zap.Bool("stderr", execConfig.AttachStderr),
		zap.Bool("tty", execConfig.Tty),
	)

	execResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return err
	}
	execID := execResp.ID

	hj, err := cli.ContainerExecAttach(ctx, execID, types.ExecStartCheck{Tty: config.Tty})
	if err != nil {
		return err
	}
	defer hj.Close()

	var g errgroup.Group

	if execConfig.AttachStdin {
		g.Go(func() error {
			defer func() {
				if cerr := hj.CloseWrite(); cerr != nil {
					logger.Warn("dexec hj.CloseWrite error",
						zap.Error(cerr),
					)
				}
			}()

			return copyFunc(hj.Conn, config.Stdin)()
		}) // Exits when stdin or the connection closes.
	}

	if execConfig.AttachStdout {
		g.Go(copyFunc(config.Stdout, hj.Conn)) // Exits when stdout or the connection closes.
	}

	if execConfig.AttachStderr {
		g.Go(copyFunc(config.Stderr, hj.Reader)) // Exits when stderr or the connection closes.
	}

	if werr := g.Wait(); werr != nil {
		logger.Warn("dexec wait error",
			zap.Error(werr),
		)
	}

	return waitForExit(ctx, cli, execID)
}

func copyFunc(w io.Writer, r io.Reader) func() error {
	return func() error {
		_, err := io.Copy(w, r)
		return err
	}
}

func waitForExit(ctx context.Context, cli client.CommonAPIClient, execID string) error {
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := cli.ContainerExecInspect(ctx, execID)
		if err != nil {
			return err
		}

		if resp.Running {
			time.Sleep(10 * time.Millisecond) // Arbitrary.
			continue
		}

		if i != 0 {
			logger := ctxlog.FromContext(ctx)
			logger.Debug("dexec waitForExit",
				zap.Int("retries", i),
			)
		}

		if resp.ExitCode != 0 {
			return ExitCodeError(resp.ExitCode)
		}

		return nil
	}
}
