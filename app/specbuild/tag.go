package specbuild

import (
	"context"
	"io"
	"io/ioutil"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"go.uber.org/zap"
)

// TagImage tags an image (by name) with a given tag name. If pull is true,
// and the named image doesn't exist, then a pull is attempted.
func TagImage(ctx context.Context, cli client.ImageAPIClient, name, tag string, pull bool) error {
	ctx, logger := ctxlog.FromContextWith(ctx,
		zap.String("image_name", name),
		zap.String("image_tag", tag),
	)

	err := cli.ImageTag(ctx, name, tag)
	if err == nil {
		return nil
	}

	if !pull || !strings.Contains(err.Error(), "No such image:") {
		return err
	}

	logger.Info("image not found, pulling")

	if err = pullImage(ctx, cli, name); err != nil {
		return err
	}

	return cli.ImageTag(ctx, name, tag)
}

func pullImage(ctx context.Context, cli client.ImageAPIClient, name string) error {
	logger := ctxlog.FromContext(ctx)

	resp, err := cli.ImagePull(ctx, name, types.ImagePullOptions{})
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
