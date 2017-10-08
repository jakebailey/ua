package app

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/ctxlog"
	"github.com/jakebailey/ua/models"
	"go.uber.org/zap"
	kallax "gopkg.in/src-d/go-kallax.v1"
)

func (a *App) cleanInstance(ctx context.Context, instance *models.Instance) error {
	ctx, logger := ctxlog.FromContextWith(ctx,
		zap.String("instance_id", instance.ID.String()),
	)

	cOpts := types.ContainerRemoveOptions{RemoveVolumes: true}
	iOpts := types.ImageRemoveOptions{PruneChildren: true}

	logger.Debug("killing container",
		zap.String("container_id", instance.ContainerID),
	)

	// Send KILL, since we don't care about the state of the container anyway and it's faster
	if err := a.cli.ContainerKill(ctx, instance.ContainerID, "KILL"); err != nil {
		if !client.IsErrNotFound(err) && !strings.Contains(err.Error(), "not running") {
			logger.Warn("error killing container, will attempt to continue cleaning anyway",
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
		zap.String("image_id", instance.ImageID),
	)

	if _, err := a.cli.ImageRemove(ctx, instance.ImageID, iOpts); err != nil {
		if !client.IsErrNotFound(err) && !strings.Contains(err.Error(), "image is being used by stopped container") {
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
		FindByCleaned(false)

	logger.Debug("cleaning up instances")

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

	a.pruneImages(ctx)

	if count != 0 {
		logger.Info("cleaned up instances",
			zap.Int("count", count),
		)
	} else {
		logger.Debug("no instances to clean up")
	}
}

func (a *App) checkExpiredInstances() {
	logger := a.logger

	instanceQuery := models.NewInstanceQuery().
		FindByActive(true).
		FindByExpiresAt(kallax.Lt, time.Now())

	logger.Debug("looking for instances to expire")

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

	if count != 0 {
		logger.Info("expired instances",
			zap.Int("count", count),
		)
	} else {
		logger.Debug("no instances to expire")
	}
}

func (a *App) cleanupLeftoverInstances() {
	ctx := ctxlog.WithLogger(context.Background(), a.logger)

	instanceQuery := models.NewInstanceQuery().FindByActive(true)
	a.cleanupInstancesByQuery(ctx, instanceQuery)
}

func (a *App) cleanupInstancesByQuery(ctx context.Context, instanceQuery *models.InstanceQuery) {
	logger := ctxlog.FromContext(ctx)

	logger.Debug("looking for leftover instances")

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

	a.pruneImages(ctx)

	if count != 0 {
		logger.Info("cleaned up instances",
			zap.Int("count", count),
		)
	} else {
		logger.Debug("no instances to clean up")
	}
}

func (a *App) markAllInstancesCleanedAndInactive() {
	logger := a.logger

	instanceQuery := models.NewInstanceQuery().
		Where(kallax.Or(
			kallax.Eq(models.Schema.Instance.Active, true),
			kallax.Eq(models.Schema.Instance.Cleaned, false),
		))

	logger.Debug("marking all instances as cleaned and inactive")

	instances, err := a.instanceStore.Find(instanceQuery)
	if err != nil {
		logger.Error("error querying for uncleaned or active instances",
			zap.Error(err),
		)
		return
	}

	count := 0

	if err := instances.ForEach(func(instance *models.Instance) error {
		instance.Active = false
		instance.Cleaned = true

		if _, err := a.instanceStore.Update(instance, models.Schema.Instance.Active, models.Schema.Instance.Cleaned); err != nil {
			logger.Error("error marking instance as cleaned and inactive in database",
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

	if count != 0 {
		logger.Info("marked instances as cleaned and inactive",
			zap.Int("count", count),
		)
	} else {
		logger.Debug("no instances to mark clean and inactive")
	}
}

var pruneFilter = filters.NewArgs()

func (a *App) pruneImages(ctx context.Context) {
	// logger := ctxlog.FromContext(ctx)

	// report, err := a.cli.ImagesPrune(ctx, pruneFilter)
	// if err != nil {
	// 	logger.Error("error pruning dangling images",
	// 		zap.Error(err),
	// 	)
	// 	return
	// }

	// count := len(report.ImagesDeleted)

	// if count != 0 {
	// 	logger.Info("pruned dangling images",
	// 		zap.Int("count", count),
	// 		zap.Uint64("reclaimed", report.SpaceReclaimed),
	// 	)
	// } else {
	// 	logger.Debug("no dangling images to prune")
	// }
}
