package image

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/docker/cli/cli/command/image/build"
	"github.com/docker/docker/pkg/archive"
)

const (
	// TemplateName is the name of the Dockerfile template in
	// assignment directory.
	TemplateName = "Dockerfile.tmpl"
	// ContextSubdir is the name of the subdirectory in the assignment
	// directory containing the docker build context.
	ContextSubdir = "context"
)

var funcs = template.FuncMap{
	// Deprecated, use {{ json . | base64 }} or similar.
	"jsonBase64": func(v interface{}) (string, error) {
		buf, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(buf), nil
	},
	"json": json.Marshal,
	"xor": func(mask byte, buf []byte) []byte {
		out := make([]byte, len(buf))

		for i, b := range buf {
			out[i] = b ^ mask
		}

		return out
	},
	"base64": base64.StdEncoding.EncodeToString,
}

func createBuildContext(root string, tmplData interface{}) (io.ReadCloser, string, error) {
	tmplPath := filepath.Join(root, TemplateName)
	contextPath := filepath.Join(root, ContextSubdir)

	// If the context doesn't exist, make an empty tempdir to delete after creating the context.
	if _, err := os.Stat(contextPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, "", err
		}

		contextPath, err = ioutil.TempDir("", "image-build-empty")
		if err != nil {
			return nil, "", err
		}
		defer os.RemoveAll(contextPath)
	}

	tmpl, err := template.New(TemplateName).Funcs(funcs).Option("missingkey=error").ParseFiles(tmplPath)
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
