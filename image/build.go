package image

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"io/ioutil"
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

func Build(ctx context.Context, cli client.CommonAPIClient, path string, tag string, tmplData interface{}) (string, error) {
	buildCtx, relDockerfile, err := createBuildContext(path, tmplData)
	if err != nil {
		return "", err
	}
	defer buildCtx.Close()

	buildOptions := types.ImageBuildOptions{
		Remove:     true,
		Dockerfile: relDockerfile,
		Tags:       []string{tag},
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

func createBuildContext(root string, tmplData interface{}) (io.ReadCloser, string, error) {
	tmplPath := filepath.Join(root, TemplateName)
	contextPath := filepath.Join(root, ContextSubdir)

	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return nil, "", err
	}

	tmplBuf := &bytes.Buffer{}
	if err := tmpl.Execute(tmplBuf, tmplData); err != nil {
		return nil, "", err
	}
	dockerfileCtx := ioutil.NopCloser(tmplBuf)

	contextDir, relDockerfile, err := build.GetContextFromLocalDir(contextPath, "-")
	if err != nil {
		return nil, "", err
	}

	excludes, err := build.ReadDockerignore(contextDir)
	if err != nil {
		return nil, "", err
	}

	if err := build.ValidateContextDirectory(contextDir, excludes); err != nil {
		return nil, "", err
	}

	relDockerfile, err = archive.CanonicalTarNameForPath(relDockerfile)
	if err != nil {
		return nil, "", err
	}

	excludes = build.TrimBuildFilesFromExcludes(excludes, relDockerfile, true)
	buildCtx, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		ExcludePatterns: excludes,
	})
	if err != nil {
		return nil, "", err
	}

	return build.AddDockerfileToBuildContext(dockerfileCtx, buildCtx)
}
