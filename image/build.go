package image

import (
	"bytes"
	"html/template"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/docker/cli/cli/command/image/build"
	"github.com/docker/docker/pkg/archive"
)

const (
	TemplateName  = "Dockerfile.tmpl"
	ContextSubdir = "context"
)

func BuildContext(root string, tmplData interface{}) (io.ReadCloser, string, error) {
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
