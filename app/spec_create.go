package app

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	units "github.com/docker/go-units"
	"github.com/jakebailey/ua/app/gobuild"
	"github.com/jakebailey/ua/app/specbuild"
	"github.com/jakebailey/ua/models"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/image"
	"go.uber.org/zap"
)

func (a *App) specCreate(ctx context.Context, assignmentPath string, specData interface{}, imageTag string, containerName string) (imageID, containerID string, iCmd *models.InstanceCommand, err error) {
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
		a.autoPullMark(out.ImageName)

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

	containerID, iCmd, err = a.specCreateContainer(ctx, assignmentPath, containerName, imageID, out)
	if err != nil {
		logger.Warn("specCreate failed, attempting to remove built image")

		iOpts := types.ImageRemoveOptions{PruneChildren: true}

		// Use another context just in case the old context was cancelled.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, removeErr := a.cli.ImageRemove(ctx, imageID, iOpts); removeErr != nil {
			logger.Warn("failed to remove image",
				zap.Error(removeErr),
			)
		}
	}

	return imageID, containerID, iCmd, err
}

func (a *App) specCreateContainer(ctx context.Context, assignmentPath string, containerName string, imageID string, gen *specbuild.GenerateOutput) (containerID string, iCmd *models.InstanceCommand, err error) {
	logger := ctxlog.FromContext(ctx)

	containerConfig := &container.Config{
		Image:     imageID,
		OpenStdin: true,
		Cmd:       []string{"/bin/cat"},
		Labels: map[string]string{
			"ua.owned": "true",
		},
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
		logger.Error("error creating container",
			zap.Error(err),
		)
		return "", nil, err
	}
	containerID = c.ID

	ctx, logger = ctxlog.FromContextWith(ctx,
		zap.String("container_id", containerID),
	)

	if err := a.specCreateContainerSetup(ctx, assignmentPath, containerID, gen); err != nil {
		logger.Warn("setup failed, attempting to remove",
			zap.Error(err),
		)

		cOpts := types.ContainerRemoveOptions{RemoveVolumes: true}

		// Use another context just in case the old context was cancelled.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if kerr := a.cli.ContainerKill(ctx, containerID, "KILL"); kerr != nil {
			logger.Warn("failed to kill container",
				zap.Error(kerr),
			)
		}

		if rerr := a.cli.ContainerRemove(ctx, containerID, cOpts); rerr != nil {
			logger.Warn("failed to remove container",
				zap.Error(rerr),
			)
		}

		return "", nil, err
	}

	iCmd = &models.InstanceCommand{
		User:       gen.User,
		Cmd:        gen.Cmd,
		Env:        gen.Env,
		WorkingDir: gen.WorkingDir,
	}

	// This somewhat correct, but the logic for which command to run needs to
	// be fixed. (TODO)
	if gen.Init != nil && *gen.Init {
		iCmd.Cmd = append([]string{"/dev/init", "-s", "--"}, iCmd.Cmd...)
	}

	return containerID, iCmd, nil
}

func (a *App) specCreateContainerSetup(ctx context.Context, assignmentPath string, containerID string, gen *specbuild.GenerateOutput) error {
	logger := ctxlog.FromContext(ctx)

	if err := a.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	for i, ac := range gen.PostBuild {
		if ac.Action != "gobuild" {
			continue
		}
		gen.PostBuild[i].SrcPath = filepath.Join(assignmentPath, "gosrc")
		a.autoPullMark(gobuild.DockerImageName)
	}

	if err := specbuild.PerformActions(ctx, a.cli, containerID, gen.PostBuild); err != nil {
		logger.Error("error performing post-build actions, will attempt to cleanup",
			zap.Error(err),
		)
		return err
	}

	if err := a.cli.NetworkDisconnect(ctx, "bridge", containerID, true); err != nil {
		logger.Error("error disconnecting network",
			zap.Error(err),
		)
		return err
	}

	if err := a.cli.ContainerStop(ctx, containerID, nil); err != nil {
		logger.Error("error stopping container",
			zap.Error(err),
		)
		return err
	}

	return nil
}
