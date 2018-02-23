package specbuild

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/image"
	"go.uber.org/zap"
)

// TagImage tags an image (by name) with a given tag name. If pull is true,
// and the named image doesn't exist, then a pull is attempted.
func TagImage(ctx context.Context, cli client.ImageAPIClient, name, tag string, pull bool) error {
	ctx, _ = ctxlog.FromContextWith(ctx,
		zap.String("image_name", name),
		zap.String("image_tag", tag),
	)

	if err := image.PullIfNotFound(ctx, cli, name); err != nil {
		return err
	}

	return cli.ImageTag(ctx, name, tag)
}
