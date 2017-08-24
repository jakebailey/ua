package app

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/models"
	"go.uber.org/zap"
)

func (a *App) cleanInactiveInstances() {
	ctx := context.Background()
	logger := a.logger

	logger.Info("cleaning up instances")

	instanceQuery := models.NewInstanceQuery().FindByActive(false).FindByCleaned(true)

	a.instanceMu.Lock()
	defer a.instanceMu.Unlock()

	instances, err := a.instanceStore.Find(instanceQuery)
	if err != nil {
		logger.Error("error querying for cleanable instances",
			zap.Error(err),
		)
		return
	}

	cOpts := types.ContainerRemoveOptions{RemoveVolumes: true}
	iOpts := types.ImageRemoveOptions{PruneChildren: true}

	if err := instances.ForEach(func(instance *models.Instance) error {
		logger := logger.With(zap.String("instance_id", instance.ID.String()))

		if err := a.cli.ContainerRemove(ctx, instance.ContainerID, cOpts); err != nil {
			if !client.IsErrNotFound(err) {
				logger.Error("error removing container",
					zap.Error(err),
					zap.String("container_id", instance.ContainerID),
				)
				return nil
			}

			logger.Warn("container didn't exist, continuing",
				zap.String("container_id", instance.ContainerID),
			)
		}

		if _, err := a.cli.ImageRemove(ctx, instance.ImageID, iOpts); err != nil {
			if !client.IsErrNotFound(err) {
				logger.Error("error removing image",
					zap.Error(err),
					zap.String("image_id", instance.ImageID),
				)
				return nil
			}

			logger.Warn("image didn't exist, continuing",
				zap.String("image_id", instance.ImageID),
			)
		}

		instance.Cleaned = true

		if _, err := a.instanceStore.Update(instance, models.Schema.Instance.Cleaned); err != nil {
			logger.Error("error marking instance as cleaned in database",
				zap.Error(err),
			)
		}

		return nil
	}); err != nil {
		logger.Error("error while looping over instances",
			zap.Error(err),
		)
	}

	// TODO: call ImagesPrune?
}
