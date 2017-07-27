package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/jakebailey/ua/image"
)

type ImageBuilder struct {
	root string
}

func NewImageBuilder(root string) *ImageBuilder {
	return &ImageBuilder{root: root}
}

func (b *ImageBuilder) Build(ctx context.Context, cli *client.Client, assignment string, tmplData interface{}) (string, error) {
	root := filepath.Join(b.root, assignment)

	buildCtx, relDockerfile, err := image.BuildContext(root, tmplData)
	if err != nil {
		return "", err
	}
	defer buildCtx.Close()

	buildOptions := types.ImageBuildOptions{
		Remove:     true,
		Dockerfile: relDockerfile,
		Tags:       []string{fmt.Sprintf("%s-%d", assignment, time.Now().Unix())},
	}

	response, err := cli.ImageBuild(ctx, buildCtx, buildOptions)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	id, err := getIDFromBody(response.Body)
	if err != nil {
		return "", err
	}

	log.Println("created image", id)
	return id, nil
}

func getIDFromBody(body io.Reader) (string, error) {
	dec := json.NewDecoder(body)
	for {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		if jm.Aux != nil {
			var result types.BuildResult
			if err := json.Unmarshal(*jm.Aux, &result); err != nil {
				return "", err
			}
			return result.ID, nil
		}
	}

	return "", errors.New("ID not found")
}

type dummyReadCloser struct {
	io.Reader
}

func (d dummyReadCloser) Close() error { return nil }

var _ io.ReadCloser = dummyReadCloser{}
