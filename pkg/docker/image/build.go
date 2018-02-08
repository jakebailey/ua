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
func Build(ctx context.Context, cli client.CommonAPIClient, tag string, dockerfile string, contextPath string) (string, error) {
	buildCtx, relDockerfile, err := createBuildContext(dockerfile, contextPath)
	if err != nil {
		return "", err
	}
	defer buildCtx.Close()

	buildOptions := types.ImageBuildOptions{
		Remove:     true,
		Dockerfile: relDockerfile,
		Tags:       []string{tag},
	}

	response, err := cli.ImageBuild(ctx, buildCtx, buildOptions)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

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