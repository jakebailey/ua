package image

import (
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/cli/cli/command/image/build"
	"github.com/docker/docker/pkg/archive"
)

func createBuildContext(dockerfile string, contextPath string) (buildCtx io.ReadCloser, relDockerfile string, err error) {
	// If the context doesn't exist, make an empty tempdir to delete after creating the context.
	if _, terr := os.Stat(contextPath); terr != nil {
		if !os.IsNotExist(terr) {
			return nil, "", terr
		}

		contextPath, terr = ioutil.TempDir("", "image-build-empty")
		if terr != nil {
			return nil, "", terr
		}

		defer func() {
			if cerr := os.RemoveAll(contextPath); cerr != nil && err == nil {
				err = cerr
			}
		}()
	}

	dockerfileCtx := ioutil.NopCloser(strings.NewReader(dockerfile))

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

	relDockerfile = archive.CanonicalTarNameForPath(relDockerfile)

	buildCtx, err = archive.TarWithOptions(contextDir, &archive.TarOptions{
		ExcludePatterns: build.TrimBuildFilesFromExcludes(excludes, relDockerfile, true),
	})
	if err != nil {
		return nil, "", err
	}

	return build.AddDockerfileToBuildContext(dockerfileCtx, buildCtx)
}
