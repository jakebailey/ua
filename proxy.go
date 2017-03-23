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

	exit := make(chan struct{})
	once := sync.Once{}
	done := func() {
		once.Do(func() {
			close(exit)
		})
	}

	go proxyInput(ctx, done, id, cli, conn, hj)
	go proxyOutput(ctx, done, id, cli, conn, hj)

	<-exit
	return nil
}

func proxyInput(ctx context.Context, done func(), id string, cli *client.Client, conn *websocket.Conn, hj types.HijackedResponse) {
	defer func() {
		log.Printf("%v: stdin proxy stopping", id[:10])
		done()
	}()
	log.Printf("%v: stdin proxy starting", id[:10])

	var buf []interface{}
	for {
		err := conn.ReadJSON(&buf)
		if err != nil {
			log.Println(err)
			return
		}

		switch buf[0] {
		case "stdin":
			if _, err := hj.Conn.Write([]byte(buf[1].(string))); err != nil {
				log.Println(err)
				return
			}
		case "set_size":
			height := uint(buf[1].(float64))
			width := uint(buf[2].(float64))

			log.Printf("resizing %v to %vx%v", id, height, width)
			if err := cli.ContainerResize(ctx, id, types.ResizeOptions{
				Height: height,
				Width:  width,
			}); err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func proxyOutput(ctx context.Context, done func(), id string, cli *client.Client, conn *websocket.Conn, hj types.HijackedResponse) {
	var m sync.Mutex

	scan := func(reader io.Reader, name string) {
		defer func() {
			log.Printf("%v: %v proxy stopping", id[:10], name)
			done()
		}()
		log.Printf("%v: %v proxy starting", id[:10], name)

		s := bufio.NewScanner(reader)
		s.Split(ScanRunesGreedy)

		for s.Scan() {
			if err := s.Err(); err != nil {
				log.Printf("scanner error: %s", err)
				return
			}

			m.Lock()
			err := conn.WriteJSON([]string{"stdout", s.Text()})
			m.Unlock()

			if err != nil {
				log.Println(err)
				return
			}
		}
	}

	go scan(hj.Conn, "stdout")
	go scan(hj.Reader, "stderr")
}
