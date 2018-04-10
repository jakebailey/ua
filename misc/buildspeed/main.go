package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/jakebailey/ua/pkg/simplecrypto"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

var args = struct {
	Addr      string `arg:"--addr" help:"address of the uAssign server, ex: term.example.edu:1234"`
	Protocol  string `arg:"--proto" help:"websocket protocol, ws or wss"`
	httpProto string `arg:"-"`

	Key string `arg:"--key,env:UA_AES_KEY,required" help:"encryption key, base64 encoded"`
	key []byte `arg:"-"`

	Num int `arg:"--num" help:"number of samples"`
}{
	Addr:     "localhost:8000",
	Protocol: "ws",
	Num:      50,
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

func cleanup(ctx context.Context, s spec) error {
	url := args.httpProto + "://" + args.Addr + "/spec/clean?async=false"

	resp, err := doPost(url, s)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
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

	sp := spec{
		SpecID: uuid.NewV4().String(),
		// AssignmentName: "00_basic_use_old",
		// Data: map[string]string{
		// 	"filename": "foobar",
		// 	"secret":   "foobar",
		// },
		AssignmentName: "fs.rm_dir_old",
		Data: map[string]string{
			"secret":     "foobar",
			"oldDirname": "oldDirname",
			"dirname":    "dirname",
		},
	}

	ctx := contextWithSigterm(context.Background())

	for i := 0; i < args.Num; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		before := time.Now()
		if _, err := postSpec(sp); err != nil {
			log.Fatal(err)
		}
		fmt.Println(time.Since(before).Seconds())

		cleanup(ctx, sp)

		time.Sleep(time.Second)
	}
}
