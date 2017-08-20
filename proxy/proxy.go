package proxy

import (
	"bufio"
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
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
	log := zerolog.Ctx(ctx)

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

	log.Info().Msg("proxying")

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
		log := zerolog.Ctx(ctx)

		defer log.Debug().Msg("stdin proxy stopping")
		log.Debug().Msg("stdin proxy starting")

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
					log.Error().Interface("bad_height", buf[1]).Msg("invalid height")
					continue
				}
				height := uint(hFloat)

				wFloat, wOk := buf[2].(float64)
				if !wOk {
					log.Error().Interface("bad_width", buf[2]).Msg("invalid width")
					continue
				}
				width := uint(wFloat)

				log.Debug().
					Uint("height", height).
					Uint("width", width).
					Msg("resizing container")

				if err := cli.ContainerResize(ctx, id, types.ResizeOptions{
					Height: height,
					Width:  width,
				}); err != nil {
					return err
				}
			default:
				log.Warn().Interface("command", buf[0]).Msg("unknown command")
			}
		}
	}
}

func proxyOutputFunc(ctx context.Context, id string, conn Conn, reader io.Reader, name string) func() error {
	return func() error {
		log := zerolog.Ctx(ctx)

		defer log.Debug().Msgf("%s proxy stopping", name)
		log.Debug().Msgf("%s proxy starting", name)

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
