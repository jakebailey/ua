package app

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	units "github.com/docker/go-units"
	"github.com/jakebailey/ua/app/specbuild"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/image"
	"go.uber.org/zap"
)

func (a *App) specCreate(ctx context.Context, assignmentPath string, specData interface{}, imageTag string) (imageID, containerID string, iCmd *models.InstanceCommand, err error) {
	logger := ctxlog.FromContext(ctx)

	out, err := specbuild.Generate(ctx, assignmentPath, specData)
	if err != nil {
		return "", "", nil, err
	}

	switch {
	case out.ImageName != "":
		if err = specbuild.TagImage(ctx, a.cli, out.ImageName, imageTag, true); err != nil {
			return "", "", nil, err
		}

		imageID = imageTag

	case out.Dockerfile != "":
		contextPath := filepath.Join(assignmentPath, "context")

		imageID, err = image.Build(ctx, a.cli, imageTag, out.Dockerfile, contextPath)
		if err != nil {
			return "", "", nil, err
		}

	default:
		logger.Error("not enough info to build image (image name, dockerfile, etc)")
		return "", "", nil, errors.New("TODO: no way to build image")
	}

	ctx, logger = ctxlog.FromContextWith(ctx,
		zap.String("image_id", imageID),
	)

	containerID, iCmd, err = a.specCreateContainer(ctx, imageID, out)
	if err != nil {
		logger.Warn("specCreate failed, attempting to remove built image")

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

func (a *App) specCreateContainer(ctx context.Context, imageID string, gen *specbuild.GenerateOutput) (containerID string, iCmd *models.InstanceCommand, err error) {
	logger := ctxlog.FromContext(ctx)

	containerName := ""

	containerConfig := &container.Config{
		Image:     imageID,
		OpenStdin: true,
		Cmd:       []string{"/bin/cat"},
	}
	hostConfig := &container.HostConfig{
		Init: gen.Init,
	}

	if !a.disableLimits {
		hostConfig.Resources.CPUShares = 2
		hostConfig.Resources.Memory = 16 * units.MiB
		hostConfig.Resources.MemoryReservation = 4 * units.MiB
		hostConfig.StorageOpt = map[string]string{
			"size": "500M",
		}
	}

	c, err := a.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, containerName)
	if err != nil {
		return "", nil, err
	}
	containerID = c.ID

	ctx, logger = ctxlog.FromContextWith(ctx,
		zap.String("container_id", containerID),
	)

	if err = a.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return "", nil, err
	}

	err = specbuild.PerformActions(ctx, a.cli, containerID, gen.PostBuild)
	if err != nil {
		logger.Error("error performing post-build actions, will attempt to cleanup",
			zap.Error(err),
		)
	}

	if discErr := a.cli.NetworkDisconnect(ctx, "bridge", containerID, true); discErr != nil {
		logger.Error("error disconnecting network",
			zap.Error(discErr),
		)
	}

	if stopErr := a.cli.ContainerStop(ctx, containerID, nil); stopErr != nil {
		logger.Error("error disconnecting network",
			zap.Error(stopErr),
		)
		return "", nil, stopErr
	}

	iCmd = &models.InstanceCommand{
		User:       gen.User,
		Cmd:        gen.Cmd,
		Env:        gen.Env,
		WorkingDir: gen.WorkingDir,
	}

	return containerID, iCmd, err
}
