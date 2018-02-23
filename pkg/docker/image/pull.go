package image

import (
	"context"
	"io"
	"io/ioutil"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"go.uber.org/zap"
)

// Pull pulls a docker image by ref name.
func Pull(ctx context.Context, cli client.ImageAPIClient, ref string) error {
	distRef, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return err
	}
	ref = distRef.String()

	resp, err := cli.ImagePull(ctx, ref, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	logger := ctxlog.FromContext(ctx)

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

// PullIfNotFound pulls an image using Pull if the image doesn't already exist.
func PullIfNotFound(ctx context.Context, cli client.ImageAPIClient, ref string) error {
	_, _, err := cli.ImageInspectWithRaw(ctx, ref)
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "No such image:") {
		return err
	}

	return Pull(ctx, cli, ref)
}
