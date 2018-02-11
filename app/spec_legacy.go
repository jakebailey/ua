package app

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	units "github.com/docker/go-units"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/image"
	"go.uber.org/zap"
)

func (a *App) specLegacyCreate(ctx context.Context, assignmentPath string, specData interface{}, imageTag string) (imageID, containerID string, iCmd *models.InstanceCommand, err error) {
	logger := ctxlog.FromContext(ctx)

	imageID, err = image.BuildLegacy(ctx, a.cli, assignmentPath, imageTag, specData)
	if err != nil {
		logger.Error("error building image",
			zap.Error(err),
		)
		return "", "", nil, err
	}

	ctx, logger = ctxlog.FromContextWith(ctx,
		zap.String("image_id", imageID),
	)

	containerID, iCmd, err = a.specLegacyCreateContainer(ctx, imageID)
	if err != nil {
		logger.Warn("specLegacyCreate failed, attempting to remove built image")

		iOpts := types.ImageRemoveOptions{PruneChildren: true}

		// Use another context just in case the old context was cancelled.
		if _, removeErr := a.cli.ImageRemove(context.Background(), imageID, iOpts); err != nil {
			logger.Error("failed to remove image",
				zap.Error(removeErr),
			)
		}
	}

	return imageID, containerID, iCmd, err
}

func (a *App) specLegacyCreateContainer(ctx context.Context, imageID string) (containerID string, iCmd *models.InstanceCommand, err error) {
	logger := ctxlog.FromContext(ctx)

	containerName := ""
	truth := true

	containerConfig := container.Config{
		Tty:       true,
		OpenStdin: true,
		Image:     imageID,
	}
	hostConfig := container.HostConfig{
		Init:        &truth,
		NetworkMode: "none",
	}

	if initCmd, ok := image.GetLabel(ctx, a.cli, imageID, "ua.initCmd"); ok {
		containerConfig.Cmd = []string{"/dev/init", "-s", "--", "/bin/sh", "-c", initCmd}
	}

	if !a.disableLimits {
		hostConfig.Resources.CPUShares = 2
		hostConfig.Resources.Memory = 16 * units.MiB
		hostConfig.Resources.MemoryReservation = 4 * units.MiB
		hostConfig.StorageOpt = map[string]string{
			"size": "500M",
		}
	}

	c, createErr := a.cli.ContainerCreate(ctx, &containerConfig, &hostConfig, nil, containerName)
	if createErr != nil {
		logger.Error("error creating container",
			zap.Error(createErr),
		)
		return "", nil, createErr
	}
	containerID = c.ID

	ctx, logger = ctxlog.FromContextWith(ctx,
		zap.String("container_id", containerID),
	)

	iCmd, err = a.specLegacyCreateCmd(ctx, containerID)

	if err != nil {
		logger.Warn("specLegacyCreate failed, attempting to remove created container")

		cOpts := types.ContainerRemoveOptions{RemoveVolumes: true}

		// Use another context just in case the old context was cancelled.
		if err := a.cli.ContainerRemove(context.Background(), containerID, cOpts); err != nil {
			logger.Error("failed to remove container",
				zap.Error(err),
			)
		}
	}

	return containerID, iCmd, nil
}

func (a *App) specLegacyCreateCmd(ctx context.Context, containerID string) (*models.InstanceCommand, error) {
	info, err := a.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	iCmd := &models.InstanceCommand{
		User: info.Config.User,
	}

	if userCmd, ok := info.Config.Labels["ua.userCmd"]; ok {
		iCmd.Cmd = []string{"/dev/init", "-s", "--", "/bin/sh", "-c", userCmd}
	} else {
		iCmd.Cmd = []string{"/dev/init", "-s", "--"}
		iCmd.Cmd = append(iCmd.Cmd, info.Config.Entrypoint...)
		iCmd.Cmd = append(iCmd.Cmd, info.Config.Cmd...)
	}

	return iCmd, nil
}
