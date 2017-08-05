package proxy

import (
	"bufio"
	"context"
	"io"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/sync/errgroup"
)

type Conn interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	Close() error
	IsClose(error) bool
}

// Proxy attaches to a docker container and proxies its stdin/out/err
// over a websocket using the terminado protocol.
func Proxy(ctx context.Context, id string, conn Conn, cli client.CommonAPIClient) error {
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

	log.Printf("%v: proxying", id[:10])

	g, ctx := errgroup.WithContext(ctx)

	g.Go(proxyInputFunc(ctx, id, conn, cli, hj.Conn))
	g.Go(proxyOutputFunc(id, conn, hj.Conn, "stdout"))
	g.Go(proxyOutputFunc(id, conn, hj.Reader, "stderr"))

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
		defer log.Printf("%v: stdin proxy stopping", id[:10])
		log.Printf("%v: stdin proxy starting", id[:10])

		var buf []interface{}
		for {
			err := conn.ReadJSON(&buf)
			if err != nil {
				return err
			}

			switch buf[0] {
			case "stdin":
				if _, err := writer.Write([]byte(buf[1].(string))); err != nil {
					log.Println(err)
					return err
				}
			case "set_size":
				height := uint(buf[1].(float64)) // TODO: properly error check these
				width := uint(buf[2].(float64))

				log.Printf("resizing %v to %vx%v", id[:10], height, width)
				if err := cli.ContainerResize(ctx, id, types.ResizeOptions{
					Height: height,
					Width:  width,
				}); err != nil {
					log.Println(err)
					return err
				}
			default:
				log.Printf("unknown command: %v", buf[0])
			}
		}
	}
}

func proxyOutputFunc(id string, conn Conn, reader io.Reader, name string) func() error {
	return func() error {
		defer log.Printf("%v: %v proxy stopping", id[:10], name)
		log.Printf("%v: %v proxy starting", id[:10], name)

		s := bufio.NewScanner(reader)
		s.Split(ScanRunesGreedy)

		for s.Scan() {
			if err := s.Err(); err != nil {
				log.Printf("scanner error: %s", err)
				return err
			}

			if err := conn.WriteJSON([]string{"stdout", s.Text()}); err != nil {
				log.Println(err)
				return err
			}
		}

		return io.EOF
	}
}
