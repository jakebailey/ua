package image

import (
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/cli/cli/command/image/build"
	"github.com/docker/docker/pkg/archive"
)

func createBuildContext(dockerfile string, contextPath string) (io.ReadCloser, string, error) {
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
