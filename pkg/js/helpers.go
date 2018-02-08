package js

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrPathEscapes is returns when a given path would escape, that is,
// when joined with a path, something outside that path may be accessed.
var ErrPathEscapes = errors.New("given path would escape")

// PathsModuleLoader returns a function which searches a list of paths in order,
// returning the first accessible JS module's source code, or an error.
func PathsModuleLoader(paths ...string) func(name string) ([]byte, error) {
	return func(name string) (out []byte, e error) {
		name = path.Clean(name)

		if path.IsAbs(name) || strings.HasPrefix(name, "../") {
			return nil, ErrPathEscapes
		}

		name = filepath.FromSlash(name)

		for _, base := range paths {
			p := filepath.Join(base, name)

			if buf, err := ioutil.ReadFile(p); err == nil {
				return buf, nil
			}

			p += ".js"

			if buf, err := ioutil.ReadFile(p); err == nil {
				return buf, nil
			}
		}

		return nil, os.ErrNotExist
	}
}

// PathsFileReader returns a function which searches a list of paths in order,
// returning the first accessible file, or an error.
func PathsFileReader(paths ...string) func(filename string) ([]byte, error) {
	return func(filename string) ([]byte, error) {
		filename = path.Clean(filename)

		if path.IsAbs(filename) || strings.HasPrefix(filename, "../") {
			return nil, ErrPathEscapes
		}

		filename = filepath.FromSlash(filename)

		for _, base := range paths {
			p := filepath.Join(base, filename)

			buf, err := ioutil.ReadFile(p)
			if err != nil {
				continue
			}

			return buf, nil
		}

		return nil, os.ErrNotExist
	}
}
