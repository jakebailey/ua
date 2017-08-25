package app

import (
	"context"
	"time"

	"github.com/jakebailey/ua/ctxlog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/models"
	"go.uber.org/zap"
	kallax "gopkg.in/src-d/go-kallax.v1"
)

func (a *App) cleanInstance(ctx context.Context, instance *models.Instance) error {
	logger := ctxlog.FromContext(ctx).With(zap.String("instance_id", instance.ID.String()))
	cOpts := types.ContainerRemoveOptions{RemoveVolumes: true}
	iOpts := types.ImageRemoveOptions{PruneChildren: true}

	logger.Debug("killing container",
		zap.String("container_id", instance.ContainerID),
	)

	// Send KILL, since we don't care about the state of the container anyway and it's faster
	if err := a.cli.ContainerKill(ctx, instance.ContainerID, "KILL"); err != nil {
		if !client.IsErrNotFound(err) {
			logger.Warn("error stopping container, will attempt to continue cleaning anyway",
				zap.Error(err),
				zap.String("container_id", instance.ContainerID),
			)
		}
	}

	logger.Debug("removing container",
		zap.String("container_id", instance.ContainerID),
	)

	if err := a.cli.ContainerRemove(ctx, instance.ContainerID, cOpts); err != nil {
		if !client.IsErrNotFound(err) {
			logger.Error("error removing container",
				zap.Error(err),
				zap.String("container_id", instance.ContainerID),
			)
			return err
		}

		logger.Warn("container didn't exist, continuing",
			zap.String("container_id", instance.ContainerID),
		)
	}

	logger.Debug("removing image",
		zap.String("container_id", instance.ImageID),
	)

	if _, err := a.cli.ImageRemove(ctx, instance.ImageID, iOpts); err != nil {
		if !client.IsErrNotFound(err) {
			logger.Error("error removing image",
				zap.Error(err),
				zap.String("image_id", instance.ImageID),
			)
			return err
		}

		logger.Warn("image didn't exist, continuing",
			zap.String("image_id", instance.ImageID),
		)
	}

	instance.Active = false
	instance.Cleaned = true

	if _, err := a.instanceStore.Update(instance, models.Schema.Instance.Active, models.Schema.Instance.Cleaned); err != nil {
		logger.Error("error marking instance as cleaned in database",
			zap.Error(err),
		)
		return err
	}

	return nil
}

func (a *App) cleanInactiveInstances() {
	logger := a.logger
	ctx := ctxlog.WithLogger(context.Background(), logger)

	instanceQuery := models.NewInstanceQuery().
		FindByActive(false).
		FindByCleaned(true)

	logger.Info("cleaning up instances")

	instances, err := a.instanceStore.Find(instanceQuery)
	if err != nil {
		logger.Error("error querying for cleanable instances",
			zap.Error(err),
		)
		return
	}

	count := 0

	if err := instances.ForEach(func(instance *models.Instance) error {
		if err := a.cleanInstance(ctx, instance); err != nil {
			logger.Error("error cleaning instance",
				zap.Error(err),
				zap.String("instance_id", instance.ID.String()),
			)
			return nil
		}

		count++

		return nil
	}); err != nil {
		logger.Error("error while looping over instances",
			zap.Error(err),
		)
	}

	logger.Info("cleaned up instances",
		zap.Int("count", count),
	)

	// TODO: call ImagesPrune?
}

func (a *App) checkExpiredInstances() {
	logger := a.logger

	instanceQuery := models.NewInstanceQuery().
		FindByActive(true).
		FindByExpiresAt(kallax.Lt, time.Now())

	logger.Info("looking for instances to expire")

	instances, err := a.instanceStore.Find(instanceQuery)
	if err != nil {
		logger.Error("error querying for expired instances",
			zap.Error(err),
		)
		return
	}

	count := 0

	if err := instances.ForEach(func(instance *models.Instance) error {
		instance.Active = false

		if _, err := a.instanceStore.Update(instance, models.Schema.Instance.Active); err != nil {
			logger.Error("error marking instance as inactive in database",
				zap.Error(err),
			)
		}

		count++

		return nil
	}); err != nil {
		logger.Error("error while looping over instances",
			zap.Error(err),
		)
	}

	logger.Info("expired instances",
		zap.Int("count", count),
	)
}

func (a *App) cleanupLeftoverInstances() {
	logger := a.logger
	ctx := ctxlog.WithLogger(context.Background(), logger)

	instanceQuery := models.NewInstanceQuery().
		FindByActive(true)

	logger.Info("looking for leftover instances")

	instances, err := a.instanceStore.Find(instanceQuery)
	if err != nil {
		logger.Error("error querying for leftover instances",
			zap.Error(err),
		)
		return
	}

	count := 0

	if err := instances.ForEach(func(instance *models.Instance) error {
		if err := a.cleanInstance(ctx, instance); err != nil {
			logger.Error("error cleaning instance",
				zap.Error(err),
				zap.String("instance_id", instance.ID.String()),
			)
			return nil
		}

		count++

		return nil
	}); err != nil {
		logger.Error("error while looping over instances",
			zap.Error(err),
		)
	}

	logger.Info("cleaned up instances",
		zap.Int("count", count),
	)
}
