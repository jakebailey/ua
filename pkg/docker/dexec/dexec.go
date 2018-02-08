package dexec

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
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
func Exec(ctx context.Context, cli client.CommonAPIClient, containerID string, config Config) error {

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
			defer hj.CloseWrite()
			_, err := io.Copy(hj.Conn, config.Stdin)
			return err
		})
	}

	if execConfig.AttachStdout {
		g.Go(func() error {
			_, err := io.Copy(config.Stdout, hj.Conn)
			return err
		})
	}

	if execConfig.AttachStderr {
		g.Go(func() error {
			_, err := io.Copy(config.Stderr, hj.Reader)
			return err
		})
	}

	if err := g.Wait(); err != nil {
		log.Println(err)
	}

	resp, err := cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		return err
	}

	if resp.ExitCode != 0 {
		return ExitCodeError(resp.ExitCode)
	}

	return nil
}
