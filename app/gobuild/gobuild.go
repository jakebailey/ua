package gobuild

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"go.uber.org/zap"
)

// DockerImageName is the image name for the static builder on Docker Hub.
var DockerImageName = "jakebailey/gobuild-static"

// Options control the Go build.
type Options struct {
	// SrcPath is the path to the Go source. This is mounted in the builder
	// as $GOPATH/src.
	SrcPath string
	// Packages is a list of package names to build.
	Packages []string
	// LDFlags is a string which is inserted into the Go build's ldflags arg.
	LDFlags string
}

// Build builds Go completely static binaries in a docker container, and
// returns an io.Reader, which contains a gzipped tarball of the binaries.
func Build(ctx context.Context, cli client.CommonAPIClient, options Options) (io.Reader, error) {
	logger := ctxlog.FromContext(ctx)

	logger.Debug("gobuild",
		zap.Any("options", options),
	)

	if err := maybePull(ctx, cli); err != nil {
		return nil, err
	}

	containerConfig := &container.Config{
		Image:        DockerImageName,
		Cmd:          options.Packages,
		Env:          []string{"GO_LDFLAGS=" + options.LDFlags},
		OpenStdin:    true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}
	hostConfig := &container.HostConfig{
		AutoRemove: true,
	}

	c, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, "")
	if err != nil {
		logger.Error("error creating gobuild container",
			zap.Error(err),
		)
		return nil, err
	}
	containerID := c.ID

	ctx, logger = ctxlog.FromContextWith(ctx,
		zap.String("container_id", containerID),
	)

	attachOptions := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}

	hj, err := cli.ContainerAttach(ctx, containerID, attachOptions)
	if err != nil {
		logger.Error("error attaching to gobuild container",
			zap.Error(err),
		)

		tryContainerRemove(ctx, cli, containerID)

		return nil, err
	}
	defer hj.Close()

	resultC, errC := cli.ContainerWait(ctx, containerID, container.WaitConditionRemoved)

	if err := cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		logger.Error("error starting gobuild container",
			zap.Error(err),
		)

		tryContainerRemove(ctx, cli, containerID)

		return nil, err
	}

	go func() {
		defer func() {
			if err := hj.CloseWrite(); err != nil {
				logger.Warn("error closing gobuild stdin",
					zap.Error(err),
				)
			}
		}()

		source, err := archive.Tar(options.SrcPath, archive.Uncompressed)
		if err != nil {
			logger.Error("error tarring go source",
				zap.Error(err),
			)
			return
		}

		defer func() {
			if err := source.Close(); err != nil {
				logger.Warn("error closing gobuild source",
					zap.Error(err),
				)
			}
		}()

		if _, err := io.Copy(hj.Conn, source); err != nil {
			logger.Warn("error copying gobuild stdin",
				zap.Error(err),
			)
		}
	}() // Exits when hj.Conn closes, or the source code has been sent.

	stderr := &bytes.Buffer{}
	stdout := &bytes.Buffer{}

	go func() {
		// This is probably the most evil thing I've ever seen inside Docker.
		// For some stupid reason, stdout and stderr are multiplexed, but only
		// for this specific type of attach. Hours of debugging later, I read
		// enough of the Docker CLI source code (hijack.go) and decided to try
		// it after seeing such a weird function signature.
		//
		// Nothing could be further than a "standard copy" than this. !@#$%!
		if _, err := stdcopy.StdCopy(stdout, stderr, hj.Reader); err != nil {
			logger.Error("stdcopy error",
				zap.Error(err),
			)
		}
	}() // Exits when hj.Reader returns EOF.

	select {
	case result := <-resultC:
		if result.Error != nil {
			return nil, fmt.Errorf("%s", result.Error.Message)
		}

		if result.StatusCode != 0 {
			return nil, fmt.Errorf("gobuild: status code %d\n%s", result.StatusCode, stderr.String())
		}

	case err := <-errC:
		logger.Error("gobuild errC",
			zap.Error(err),
		)
		return nil, err
	}

	return stdout, nil
}

func tryContainerRemove(ctx context.Context, cli client.CommonAPIClient, containerID string) {
	logger := ctxlog.FromContext(ctx)

	cOpts := types.ContainerRemoveOptions{RemoveVolumes: true}
	rctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if err := cli.ContainerKill(ctx, containerID, "KILL"); err != nil {
		logger.Warn("error killing gobuild container",
			zap.Error(err),
		)
	}

	if err := cli.ContainerRemove(rctx, containerID, cOpts); err != nil {
		logger.Warn("error removing gobuild container",
			zap.Error(err),
		)
	}
}

// TODO: Deduplicate this code, which is nearly identical to specbuid's pull
// function.
func maybePull(ctx context.Context, cli client.CommonAPIClient) error {
	logger := ctxlog.FromContext(ctx)

	_, _, err := cli.ImageInspectWithRaw(ctx, DockerImageName)
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "No such image:") {
		return err
	}

	resp, err := cli.ImagePull(ctx, DockerImageName, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	if _, err := io.Copy(ioutil.Discard, resp); err != nil {
		logger.Warn("error discarding image pull status",
			zap.Error(err),
		)
	}

	if err := resp.Close(); err != nil {
		logger.Warn("error closing image pull response",
			zap.Error(err),
		)
		return err
	}

	return nil
}
