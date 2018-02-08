package image

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"text/template"

	"github.com/docker/docker/client"
)

const (
	// LegacyTemplateName is the name of the Dockerfile template in
	// assignment directory.
	LegacyTemplateName = "Dockerfile.tmpl"
	// LegacyContextSubdir is the name of the subdirectory in the assignment
	// directory containing the docker build context.
	LegacyContextSubdir = "context"
)

var legacyFuncs = template.FuncMap{
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
	"gzip": func(in []byte) ([]byte, error) {
		var out bytes.Buffer
		gz := gzip.NewWriter(&out)

		if _, err := gz.Write(in); err != nil {
			return nil, err
		}

		if err := gz.Close(); err != nil {
			return nil, err
		}

		return out.Bytes(), nil
	},
}

// BuildLegacy builds a docker image on the given docker client. The process used
// differs from the "normal" docker build process in that the Dockerfile is
// a template, and exists outside of the normal build directory. A typical
// layout looks like:
//
//     test
//     +-- context
//     |   +-- helloworld.txt
//     +-- Dockerfile.tmpl
func BuildLegacy(ctx context.Context, cli client.CommonAPIClient, path string, tag string, tmplData interface{}) (string, error) {
	tmplPath := filepath.Join(path, LegacyTemplateName)
	contextPath := filepath.Join(path, LegacyContextSubdir)

	tmpl, err := template.New(LegacyTemplateName).Funcs(legacyFuncs).Option("missingkey=error").ParseFiles(tmplPath)
	if err != nil {
		return "", err
	}

	tmplBuf := &bytes.Buffer{}
	if err := tmpl.Execute(tmplBuf, tmplData); err != nil {
		return "", err
	}
	dockerfile := tmplBuf.String()

	return Build(ctx, cli, tag, dockerfile, contextPath)
}
