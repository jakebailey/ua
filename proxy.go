package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

// ProxyContainer attaches to a docker container and proxies its stdin/out/err
// over a websocket using the terminado protocol.
func ProxyContainer(ctx context.Context, id string, cli *client.Client, conn *websocket.Conn) error {
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

	ws := &wsWrapper{c: conn}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(proxyInputFunc(ctx, id, cli, ws, hj.Conn))
	g.Go(proxyOutputFunc(ctx, id, ws, hj.Conn, "stdout"))
	g.Go(proxyOutputFunc(ctx, id, ws, hj.Reader, "stderr"))

	g.Go(func() error {
		<-ctx.Done()
		conn.Close()
		hj.Close()
		return nil
	})

	return processProxyError(g.Wait())
}

func processProxyError(err error) error {
	if err == io.EOF {
		return nil
	}

	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return nil
	}

	return err
}

func proxyInputFunc(ctx context.Context, id string, cli *client.Client, ws *wsWrapper, writer io.Writer) func() error {
	return func() error {
		defer log.Printf("%v: stdin proxy stopping", id[:10])
		log.Printf("%v: stdin proxy starting", id[:10])

		var buf []interface{}
		for {
			err := ws.ReadJSON(&buf)
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

func proxyOutputFunc(ctx context.Context, id string, ws *wsWrapper, reader io.Reader, name string) func() error {
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

			if err := ws.WriteJSON([]string{"stdout", s.Text()}); err != nil {
				log.Println(err)
				return err
			}
		}

		return io.EOF
	}
}

type wsWrapper struct {
	c  *websocket.Conn
	mu sync.Mutex
}

func (ws *wsWrapper) ReadJSON(v interface{}) error {
	return ws.c.ReadJSON(v)
}

func (ws *wsWrapper) WriteJSON(v interface{}) error {
	ws.mu.Lock()
	err := ws.c.WriteJSON(v)
	ws.mu.Unlock()
	return err
}
