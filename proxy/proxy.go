package proxy

import (
	"bufio"
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/ctxlog"
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

// Proxy attaches to a docker container and proxies its stdin/out/err
// over a websocket using the terminado protocol.
func Proxy(ctx context.Context, id string, conn Conn, cli client.CommonAPIClient) error {
	logger := ctxlog.FromContext(ctx)

	hj, err := cli.ContainerAttach(ctx, id, types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return err
	}
	defer hj.Close()

	logger.Info("proxying")

	g, ctx := errgroup.WithContext(ctx)

	g.Go(proxyInputFunc(ctx, id, conn, cli, hj.Conn))
	g.Go(proxyOutputFunc(ctx, id, conn, hj.Conn, "stdout"))
	g.Go(proxyOutputFunc(ctx, id, conn, hj.Reader, "stderr"))

	g.Go(func() error {
		<-ctx.Done()
		conn.Close()
		hj.Close()
		return nil
	})

	if err := g.Wait(); err != nil && !conn.IsClose(err) {
		return err
	}

	return nil
}

func proxyInputFunc(ctx context.Context, id string, conn Conn, cli client.CommonAPIClient, writer io.Writer) func() error {
	return func() error {
		logger := ctxlog.FromContext(ctx).With(zap.String("pipe", "stdin"))

		defer logger.Debug("proxy stopping")
		logger.Debug("stdin proxy starting")

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
					logger.Error("invalid height",
						zap.Any("bad_height", buf[1]),
					)
					continue
				}
				height := uint(hFloat)

				wFloat, wOk := buf[2].(float64)
				if !wOk {
					logger.Error("invalid width",
						zap.Any("bad_width", buf[2]),
					)
					continue
				}
				width := uint(wFloat)

				logger.Debug("resizing container",
					zap.Uint("height", height),
					zap.Uint("width", width),
				)

				if err := cli.ContainerResize(ctx, id, types.ResizeOptions{
					Height: height,
					Width:  width,
				}); err != nil {
					return err
				}
			default:
				logger.Warn("unknown command",
					zap.Any("command", buf[0]),
				)
			}
		}
	}
}

func proxyOutputFunc(ctx context.Context, id string, conn Conn, reader io.Reader, name string) func() error {
	return func() error {
		logger := ctxlog.FromContext(ctx).With(zap.String("pipe", name))

		defer logger.Debug("proxy stopping")
		logger.Debug("proxy starting")

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
