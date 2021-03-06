package dcompat

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
)

type compatClient struct {
	client.CommonAPIClient
}

func (c compatClient) ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error) {
	config = ExecConfig(c, config)
	return c.CommonAPIClient.ContainerExecCreate(ctx, container, config)
}

// Wrap wraps a Docker client with compatibility fixes.
func Wrap(cli client.CommonAPIClient) client.CommonAPIClient {
	// Avoid double wrapping.
	if _, ok := cli.(compatClient); ok {
		return cli
	}

	return compatClient{
		CommonAPIClient: cli,
	}
}

// ExecConfig converts an ExecConfig to be compatible with the provided
// client's version, and returns a new config.
//
// This function assumes that the exec will run on a "sane" configuration,
// with common utilities like sh (POSIX), cd, etc.
//
// Currently, this function does the following:
//     - Docker API 1.35 introduces exec working dirs. For versions below 1.35,
//       sh is used to cd into a directory first instead of being done by the
//       Docker daemon.
func ExecConfig(cli client.CommonAPIClient, ec types.ExecConfig) types.ExecConfig {
	if versions.LessThan(cli.ClientVersion(), "1.35") {
		if ec.WorkingDir != "" {
			var workDirCmd []string

			if ec.Tty {
				workDirCmd = []string{"sh", "-i", "-c"}
			} else {
				workDirCmd = []string{"sh", "-c"}
			}

			workDirCmd = append(workDirCmd, "cd "+ec.WorkingDir+"; exec $0 $@")
			ec.Cmd = append(workDirCmd, ec.Cmd...)
			ec.WorkingDir = ""
		}
	}

	return ec
}
