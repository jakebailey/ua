package image

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
)

// Build builds a docker image on the given docker client, given the Dockerfile
// as a string and the path to the build context.
func Build(ctx context.Context, cli client.CommonAPIClient, tag string, dockerfile string, contextPath string) (imageID string, err error) {
	buildCtx, relDockerfile, createErr := createBuildContext(dockerfile, contextPath)
	if createErr != nil {
		return "", createErr
	}
	defer func() {
		// We'd like to defer buildCtx.Close(), but doing so directly discards
		// a potential error. Instead, close it, check the error, and propagate
		// it upward if the Build would normally return a nil error.
		if cerr := buildCtx.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	buildOptions := types.ImageBuildOptions{
		Remove:     true,
		Dockerfile: relDockerfile,
		Tags:       []string{tag},
	}

	response, buildErr := cli.ImageBuild(ctx, buildCtx, buildOptions)
	if buildErr != nil {
		return "", buildErr
	}
	defer func() {
		if cerr := response.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	imageID, toRemove, err := readBuildBody(response.Body)
	if err != nil {
		return "", err
	}

	for _, i := range toRemove {
		_, err := cli.ImageRemove(ctx, i, types.ImageRemoveOptions{PruneChildren: true})
		if err != nil {
			return "", err
		}
	}

	return imageID, nil
}

func readBuildBody(body io.Reader) (imageID string, toRemove []string, err error) {
	var messages []string
	fromCount := 0

	dec := json.NewDecoder(body)
	for {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			return "", nil, err
		}

		if strings.Contains(jm.Stream, "FROM ") {
			fromCount++
		}

		if jm.Aux != nil {
			var result types.BuildResult
			if err := json.Unmarshal(*jm.Aux, &result); err != nil {
				return "", nil, err
			}

			// We've already seen an image go by, so the old imageID refers
			// to an intermediary build stage, so add it to the list of images
			// to remove and select the most recent image as the right one.
			if imageID != "" {
				toRemove = append(toRemove, imageID)
			}

			imageID = result.ID
		}

		// TODO: make this smarter
		messages = append(messages, fmt.Sprint(jm))
	}

	if imageID == "" {
		return "", nil, fmt.Errorf("build did not complete:\n%s", strings.Join(messages, "\n"))
	}

	if len(toRemove)+1 != fromCount {
		return "", nil, fmt.Errorf("some build stage failed:\n%s", strings.Join(messages, "\n"))
	}

	return imageID, toRemove, nil
}
