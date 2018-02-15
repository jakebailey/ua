package app

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	units "github.com/docker/go-units"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/pkg/ctxlog"
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

func (a *App) cleanInactiveInstances() {
	ctx := ctxlog.WithLogger(context.Background(), a.logger)

	instanceQuery := models.NewInstanceQuery().
		FindByActive(false).
		FindByCleaned(false)

	a.logger.Debug("cleaning up inactive instances")
	a.cleanupInstancesByQuery(ctx, instanceQuery)
}

func (a *App) cleanupLeftoverInstances() {
	ctx := ctxlog.WithLogger(context.Background(), a.logger)

	instanceQuery := models.NewInstanceQuery().FindByActive(true)

	a.logger.Debug("cleaning up leftover instances")
	a.cleanupInstancesByQuery(ctx, instanceQuery)
}

func (a *App) cleanupInstancesByQuery(ctx context.Context, instanceQuery *models.InstanceQuery) {
	logger := ctxlog.FromContext(ctx)

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

func (a *App) pruneDocker() {
	// Order: containers, networks, volumes, images, then build cache
	// (from docker system prune).

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := a.logger

	contReport, err := a.cli.ContainersPrune(ctx, filters.NewArgs(
		filters.Arg("label", "ua.owned=true"),
		filters.Arg("until", "24h"),
	))
	if err != nil {
		logger.Warn("error pruning containers",
			zap.Error(err),
		)
		contReport = types.ContainersPruneReport{}
	}

	netReport, err := a.cli.NetworksPrune(ctx, filters.NewArgs(
		filters.Arg("until", "24h"),
	))
	if err != nil {
		logger.Warn("error pruning networks",
			zap.Error(err),
		)
		netReport = types.NetworksPruneReport{}
	}

	volReport, err := a.cli.VolumesPrune(ctx, filters.NewArgs())
	if err != nil {
		logger.Warn("error pruning volumes",
			zap.Error(err),
		)
		volReport = types.VolumesPruneReport{}
	}

	imgReport, err := a.cli.ImagesPrune(ctx, filters.NewArgs(
		filters.Arg("dangling", "true"),
		filters.Arg("until", "24h"),
	))
	if err != nil {
		logger.Warn("error pruning images",
			zap.Error(err),
		)
		imgReport = types.ImagesPruneReport{}
	}

	spaceReclaimed := contReport.SpaceReclaimed + volReport.SpaceReclaimed + imgReport.SpaceReclaimed

	if spaceReclaimed == 0 && len(netReport.NetworksDeleted) == 0 {
		return
	}

	logger.Info("pruned docker system",
		zap.String("space_reclaimed", units.HumanSize(float64(spaceReclaimed))),
		zap.Int("containers", len(contReport.ContainersDeleted)),
		zap.Int("networks", len(netReport.NetworksDeleted)),
		zap.Int("volumes", len(volReport.VolumesDeleted)),
		zap.Int("images", len(imgReport.ImagesDeleted)),
	)
}
