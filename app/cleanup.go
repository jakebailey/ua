package app

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/models"
	"go.uber.org/zap"
	kallax "gopkg.in/src-d/go-kallax.v1"
)

func (a *App) cleanInactiveInstances() {
	ctx := context.Background()
	logger := a.logger

	instanceQuery := models.NewInstanceQuery().FindByActive(false).FindByCleaned(true)
	cOpts := types.ContainerRemoveOptions{RemoveVolumes: true}
	iOpts := types.ImageRemoveOptions{PruneChildren: true}
	stopTimeout := 10 * time.Second // default from docker CLI

	a.instanceMu.Lock()
	defer a.instanceMu.Unlock()

	logger.Info("cleaning up instances")

	instances, err := a.instanceStore.Find(instanceQuery)
	if err != nil {
		logger.Error("error querying for cleanable instances",
			zap.Error(err),
		)
		return
	}

	if err := instances.ForEach(func(instance *models.Instance) error {
		logger := logger.With(zap.String("instance_id", instance.ID.String()))

		if err := a.cli.ContainerStop(ctx, instance.ContainerID, &stopTimeout); err != nil {
			if !client.IsErrNotFound(err) {
				logger.Warn("error stopping container, will attempt to continue cleaning anyway",
					zap.Error(err),
					zap.String("container_id", instance.ContainerID),
				)
			}
		}

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

func (a *App) expireInstances() {
	logger := a.logger

	instanceQuery := models.NewInstanceQuery().
		FindByActive(true).
		FindByCleaned(false).
		FindByExpiresAt(kallax.Lt, time.Now())

	logger.Info("looking for instances to expire")

	instances, err := a.instanceStore.Find(instanceQuery)
	if err != nil {
		logger.Error("error querying for expired instances",
			zap.Error(err),
		)
		return
	}

	if err := instances.ForEach(func(instance *models.Instance) error {
		// TODO: send signal to connection to close

		instance.Active = false

		if _, err := a.instanceStore.Update(instance, models.Schema.Instance.Active); err != nil {
			logger.Error("error marking instance as inactive in database",
				zap.Error(err),
			)
		}

		return nil
	}); err != nil {
		logger.Error("error while looping over instances",
			zap.Error(err),
		)
	}
}
