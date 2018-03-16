package proxy

import (
	"bufio"
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Conn is the interface for sending proxy data to the client.
type Conn interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	Close() error
	IsClose(error) bool
}

// Command is the configuration for a proxy'd command.
type Command struct {
	User       string
	Cmd        []string
	Env        []string
	WorkingDir string
}

// Proxy attaches to a docker container and proxies its stdin/out/err
// over a websocket using the terminado protocol.
func Proxy(ctx context.Context, id string, conn Conn, cli client.CommonAPIClient, command Command) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := ctxlog.FromContext(ctx)

	execConfig := types.ExecConfig{
		User:         command.User,
		Cmd:          command.Cmd,
		Env:          command.Env,
		WorkingDir:   command.WorkingDir,
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}

	logger.Debug("creating exec",
		zap.Any("exec_config", execConfig),
	)

	execResp, err := cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return err
	}
	execID := execResp.ID

	ctx, logger = ctxlog.FromContextWith(ctx,
		zap.String("exec_id", execID),
	)

	logger.Debug("attaching to exec")

	hj, err := cli.ContainerExecAttach(ctx, execID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return err
	}
	defer hj.Close()

	logger.Info("proxying")

	g, ctx := errgroup.WithContext(ctx)

	// These exit when the context is cancelled, or the hijacked connection or
	// proxy connection close.
	g.Go(proxyInputFunc(ctx, execID, conn, cli, hj.Conn))
	g.Go(proxyOutputFunc(ctx, conn, hj.Conn, "stdout"))
	g.Go(proxyOutputFunc(ctx, conn, hj.Reader, "stderr"))

	g.Go(func() error {
		<-ctx.Done()
		hj.Close()
		return conn.Close()
	}) // Exits when the context is cancelled.

	if err := g.Wait(); err != nil && !conn.IsClose(err) {
		return err
	}

	return nil
}

func proxyInputFunc(ctx context.Context, id string, conn Conn, cli client.CommonAPIClient, writer io.Writer) func() error {
	return func() error {
		ctx, logger := ctxlog.FromContextWith(ctx,
			zap.String("pipe", "stdin"),
		)

		defer logger.Debug("proxy stopping")
		logger.Debug("proxy starting")

		var buf []interface{}
		for {
			err := conn.ReadJSON(&buf)
			if err != nil {
				return err
			}

			switch buf[0] {
			case "stdin":
				if _, err := writer.Write([]byte(buf[1].(string))); err != nil {
					return err
				}
			case "set_size":
				hFloat, hOk := buf[1].(float64)
				if !hOk {
					logger.Warn("invalid height",
						zap.Any("bad_height", buf[1]),
					)
					continue
				}
				height := uint(hFloat)

				wFloat, wOk := buf[2].(float64)
				if !wOk {
					logger.Warn("invalid width",
						zap.Any("bad_width", buf[2]),
					)
					continue
				}
				width := uint(wFloat)

				if height == 0 && width == 0 {
					logger.Warn("got zero height and width for resize")
					continue
				}

				resizeOptions := types.ResizeOptions{
					Height: height,
					Width:  width,
				}

				var i int
				var resizeErr error

				for i = 0; i < 5; i++ {
					resizeErr = cli.ContainerExecResize(ctx, id, resizeOptions)
					if resizeErr == nil {
						break
					}

					// Arbitrary. Set to 0.05 seconds for now.
					time.Sleep(50 * time.Millisecond)
				}

				if resizeErr == nil {
					if i > 1 {
						logger.Warn("multiple tries to resize exec",
							zap.Int("count", i),
						)
					}
				} else {
					logger.Error("error resizing exec",
						zap.Error(resizeErr),
					)
				}

			default:
				logger.Warn("unknown command",
					zap.Any("command", buf[0]),
				)
			}
		}
	}
}

func proxyOutputFunc(ctx context.Context, conn Conn, reader io.Reader, name string) func() error {
	return func() error {
		logger := ctxlog.FromContext(ctx).With(
			zap.String("pipe", name),
		)

		defer logger.Debug("proxy stopping")
		logger.Debug("proxy starting")

		if err := conn.WriteJSON([]string{"wipe"}); err != nil {
			return err
		}

		s := bufio.NewScanner(reader)
		s.Split(ScanRunesGreedy)

		for s.Scan() {
			if err := s.Err(); err != nil {
				return err
			}

			if err := conn.WriteJSON([]string{"stdout", s.Text()}); err != nil {
				return err
			}
		}

		return io.EOF
	}
}
