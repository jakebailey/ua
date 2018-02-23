package app

import (
	"context"
	"time"

	"github.com/jakebailey/ua/pkg/docker/image"
	"go.uber.org/zap"
)

func (a *App) autoPull() {
	ctx := context.Background()

	refsMap := a.autoPullImages.Items()
	refs := make([]string, 0, len(refsMap))
	for ref := range refsMap {
		refs = append(refs, ref)
	}

	a.logger.Debug("starting auto-pull",
		zap.Strings("refs", refs),
	)

	for _, ref := range refs {
		logger := a.logger.With(
			zap.String("ref", ref),
		)

		logger.Debug("attemping to auto-pull")

		before := time.Now()

		if err := image.Pull(ctx, a.cli, ref); err != nil {
			logger.Error("error auto-pulling image",
				zap.Error(err),
			)
			continue
		}

		logger.Info("auto-pulled image",
			zap.Duration("took", time.Since(before)),
		)
	}
}

func (a *App) autoPullMark(ref string) {
	a.autoPullImages.SetDefault(ref, nil)
}
