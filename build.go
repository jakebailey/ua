package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log"
	"path/filepath"

	"github.com/docker/cli/cli/command/image/build"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
)

const (
	TemplateName  = "Dockerfile.tmpl"
	ContextSubdir = "context"
)

type ImageBuilder struct {
	root string
}

func NewImageBuilder(root string) *ImageBuilder {
	return &ImageBuilder{root: root}
}

func (b *ImageBuilder) Build(ctx context.Context, cli *client.Client, assignment string, tmplData interface{}) (string, error) {
	root := filepath.Join(b.root, assignment)
	tmplPath := filepath.Join(root, TemplateName)
	contextPath := filepath.Join(root, ContextSubdir)

	log.Println("parsing template")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return "", err
	}

	log.Println("executing template")
	tmplBuf := &bytes.Buffer{}
	if err := tmpl.Execute(tmplBuf, tmplData); err != nil {
		return "", err
	}
	dockerfileCtx := dummyReadCloser{tmplBuf}

	log.Println("getting context from local dir")
	contextDir, relDockerfile, err := build.GetContextFromLocalDir(contextPath, "-")
	if err != nil {
		return "", err
	}

	log.Println("reading dockerignore")
	excludes, err := build.ReadDockerignore(contextDir)
	if err != nil {
		return "", err
	}

	log.Println("validating context")
	if err := build.ValidateContextDirectory(contextDir, excludes); err != nil {
		return "", err
	}

	log.Println("building archive")
	// And canonicalize dockerfile name to a platform-independent one
	relDockerfile, err = archive.CanonicalTarNameForPath(relDockerfile)
	if err != nil {
		return "", err
	}

	excludes = build.TrimBuildFilesFromExcludes(excludes, relDockerfile, true)
	buildCtx, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		ExcludePatterns: excludes,
	})
	if err != nil {
		return "", err
	}

	log.Println("adding Dockerfile to context")
	buildCtx, relDockerfile, err = build.AddDockerfileToBuildContext(dockerfileCtx, buildCtx)
	if err != nil {
		return "", err
	}

	buildOptions := types.ImageBuildOptions{
		Dockerfile: relDockerfile,
	}

	log.Println("building image")
	response, err := cli.ImageBuild(ctx, buildCtx, buildOptions)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	log.Println("decoding response")
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
