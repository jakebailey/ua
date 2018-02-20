package image

import (
	"context"

	"github.com/docker/docker/client"
)

// GetLabel gets a label from an image.
func GetLabel(ctx context.Context, cli client.CommonAPIClient, imageID string, labelName string) (string, bool) {
	ii, _, err := cli.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		return "", false
	}

	if ii.Config == nil {
		return "", false
	}

	value := ii.Config.Labels[labelName]

	if value == "" {
		return "", false
	}
	return value, true
}
