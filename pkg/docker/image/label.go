package image

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/docker/cli/cli/command/inspect"
	"github.com/docker/docker/client"
)

// GetLabel gets a label from an image.
func GetLabel(ctx context.Context, cli client.CommonAPIClient, imageID string, labelName string) (string, bool) {
	var buf bytes.Buffer

	filter := fmt.Sprintf(`{{ index .Config.Labels "%s" }}`, labelName)

	getRefFunc := func(ref string) (interface{}, []byte, error) {
		return cli.ImageInspectWithRaw(ctx, ref)
	}

	if err := inspect.Inspect(&buf, []string{imageID}, filter, getRefFunc); err != nil {
		return "", false
	}

	value := buf.String()
	value = strings.TrimSpace(value)

	if value == "" {
		return "", false
	}
	return value, true
}
