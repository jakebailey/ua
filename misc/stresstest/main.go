package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/jakebailey/ua/pkg/docker/proxy"
	"github.com/jakebailey/ua/pkg/simplecrypto"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/sync/errgroup"
)

var args = struct {
	Addr      string `arg:"--addr" help:"address of the uAssign server, ex: term.example.edu:1234"`
	Protocol  string `arg:"--proto" help:"websocket protocol, ws or wss"`
	httpProto string `arg:"-"`

	Key string `arg:"--key,env:UA_AES_KEY,required" help:"encryption key, base64 encoded"`
	key []byte `arg:"-"`

	Num int `arg:"--num" help:"number of fake users"`

	Dur     time.Duration `arg:"--dur" help:"duration of test"`
	Stagger time.Duration `arg:"--stagger" help:"time to wait between user startup"`
}{
	Addr:     "localhost:8000",
	Protocol: "ws",
	Num:      10,
	Dur:      5 * time.Minute,
	Stagger:  time.Second,
}

func randDur(center, variance time.Duration) time.Duration {
	min := center - variance
	max := center + variance

	r := rand.Int63n(int64(max - min))

	return time.Duration(r) + min
}

func doPost(url string, v interface{}) (*http.Response, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	if err := simplecrypto.EncodeJSONWriter(args.key, b, &buf); err != nil {
		return nil, err
	}

	return http.Post(url, "application/json", &buf)
}

type spec struct {
	SpecID         string      `json:"specID"`
	AssignmentName string      `json:"assignmentName"`
	Data           interface{} `json:"data"`
}

type specResp struct {
	InstanceID string `json:"instanceID"`
}

func postSpec(s spec) (string, error) {
	url := args.httpProto + "://" + args.Addr + "/spec"

	resp, err := doPost(url, s)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.Errorf("code %d, %s", resp.StatusCode, resp.Status)
	}

	var sr specResp
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", err
	}

	return sr.InstanceID, nil
}

func getWS(ctx context.Context, id string) (net.Conn, error) {
	url := args.Protocol + "://" + args.Addr + "/instance/" + id + "/ws"

	conn, br, _, err := ws.Dial(ctx, url)
	if err != nil {
		return nil, err
	}

	if br != nil {
		ws.PutReader(br)
	}

	return conn, nil
}

func cleanup(ctx context.Context, s spec) error {
	url := args.httpProto + "://" + args.Addr + "/spec/clean?async=false"

	resp, err := doPost(url, s)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

type worker struct {
	num      int
	period   time.Duration
	variance time.Duration
	spec     spec
}

func newWorker(num int) *worker {
	return &worker{
		num:      num,
		period:   5 * time.Second,
		variance: time.Second,
		spec: spec{
			SpecID:         uuid.NewV4().String(),
			AssignmentName: "00_basic_use",
			Data: map[string]string{
				"filename": "foobar",
				"secret":   "foobar",
			},
			// AssignmentName: "fs.cp",
			// Data: map[string]string{
			// 	"oldFilename": "test",
			// 	"contents":    "1234567890",
			// },
			// AssignmentName: "globbing.range",
			// Data: map[string]string{
			// 	"secret": "foobar",
			// },
		},
	}
}

func (w *worker) work(ctx context.Context, connected chan<- struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context cancelled")
		default:
		}

		if err := w.doFor(ctx, time.Minute, connected); err != nil {
			log.Println(w.num, "-", err)
			return err
		}

		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context cancelled")
		default:
		}

		dur := randDur(w.period, w.variance)
		time.Sleep(dur)

		connected = nil
	}
}

func (w *worker) doFor(ctx context.Context, d time.Duration, connected chan<- struct{}) error {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return w.do(ctx, connected)
}

func (w *worker) do(ctx context.Context, connected chan<- struct{}) (oerr error) {
	before := time.Now()
	specID, err := postSpec(w.spec)
	if err != nil {
		return errors.Wrap(err, "postSpec")
	}
	log.Printf("%d - postSpec took %v", w.num, time.Since(before))

	if connected != nil {
		connected <- struct{}{}
	}

	defer func() {
		if err := cleanup(ctx, w.spec); err != nil && oerr == nil {
			oerr = errors.Wrap(err, "cleanup")
		}
	}()

	conn, err := getWS(ctx, specID)
	if err != nil {
		return errors.Wrap(err, "getWS")
	}

	if err := w.simulate(ctx, conn); err != nil {
		return errors.Wrap(err, "simulate")
	}

	return nil
}

func (w *worker) simulate(ctx context.Context, conn net.Conn) error {
	g, ctx := errgroup.WithContext(ctx)

	commands := []string{
		"pwd\n",
		"whoami\n",
		"ls /\n",
		"echo hello\n",
	}

	g.Go(func() error {
		for {
			for _, c := range commands {
				select {
				case <-ctx.Done():
					return errors.Wrap(ctx.Err(), "context cancelled")
				default:
				}

				cmd := []string{"stdin", c}

				buf, err := json.Marshal(cmd)
				if err != nil {
					return errors.Wrap(err, "ws write json")
				}

				err = wsutil.WriteClientText(conn, buf)

				if err != nil {
					return errors.Wrap(err, "ws write")
				}

				time.Sleep(5 * time.Second)
			}
		}
	})

	g.Go(func() error {
		for {
			buf, op, err := wsutil.ReadServerData(conn)
			if err != nil {
				return err
			}

			_, _ = buf, op

			// log.Printf("%d - op(%d) : %s", w.num, op, buf)
		}

		// _, err := io.Copy(ioutil.Discard, conn)
		// return errors.Wrap(err, "ws read")
	})

	g.Go(func() error {
		<-ctx.Done()
		return errors.Wrap(conn.Close(), "conn close")
	})

	err := g.Wait()
	if err != nil {
		if proxy.WSIsClose(err) {
			return nil
		}
	}
	return err
}

func contextWithSigterm(ctx context.Context) context.Context {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()

		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

		select {
		case <-signalCh:
		case <-ctx.Done():
		}
	}()

	return ctxWithCancel
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if err := godotenv.Load(); err != nil {
		panic(err)
	}

	p := arg.MustParse(&args)

	key, err := base64.StdEncoding.DecodeString(args.Key)
	if err != nil {
		p.Fail(err.Error())
	}
	args.key = key

	if args.Protocol == "wss" {
		args.httpProto = "https"
	} else {
		args.httpProto = "http"
	}

	ctx, cancel := context.WithTimeout(context.Background(), args.Dur)
	defer cancel()

	ctx = contextWithSigterm(ctx)

	g, ctx := errgroup.WithContext(ctx)

	log.Printf("starting %d simulated users, staggered by %v", args.Num, args.Stagger)

	connected := make(chan struct{})

	for i := 0; i < args.Num; i++ {
		w := newWorker(i)

		g.Go(func() error {
			return w.work(ctx, connected)
		})

		<-connected

		time.Sleep(args.Stagger)
	}

	log.Printf("started %d simulated users, waiting", args.Num)

	log.Println(g.Wait())
}
