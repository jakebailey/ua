package main

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func buildContainer(ctx context.Context, cli *client.Client, assignment string) (string, error) {
	filename := filepath.Join("assignments", assignment, "base", "Dockerfile")
	dockerfile, err := os.Open(filename)
	if err != nil {
		return "", err
	}

	dockerfileBuf := &bytes.Buffer{}
	dockerfileBuf.ReadFrom(dockerfile)

	buildCtxBuf := &bytes.Buffer{}
	tw := tar.NewWriter(buildCtxBuf)

	hdr := &tar.Header{
		Name: "Dockerfile",
		Mode: 0600,
		Size: int64(dockerfileBuf.Len()),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return "", err
	}

	if _, err := tw.Write(dockerfileBuf.Bytes()); err != nil {
		return "", err
	}

	if err := tw.Close(); err != nil {
		return "", err
	}

	resp, err := cli.ImageBuild(ctx, buildCtxBuf, types.ImageBuildOptions{
		PullParent: true,
		// Dockerfile: "Dockerfile",
	})

	if err != nil {
		return "", err
	}

	var outBuf bytes.Buffer
	outBuf.ReadFrom(resp.Body)

	return outBuf.String(), nil
}
