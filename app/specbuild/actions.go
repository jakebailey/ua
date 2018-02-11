package specbuild

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/client"
	"github.com/jakebailey/ua/pkg/ctxlog"
	"github.com/jakebailey/ua/pkg/docker/dexec"
	"go.uber.org/zap"
)

// Action defines an action that can be performed on a running container.
type Action struct {
	Action     string
	User       string
	WorkingDir string

	// Exec action
	Cmd   []string
	Env   []string
	Stdin *string

	// Write/append option
	Contents       string
	ContentsBase64 bool
	Filename       string
}

// PerformActions performs the given actions on the specified container.
func PerformActions(ctx context.Context, cli client.CommonAPIClient, containerID string, actions []Action) error {
	logger := ctxlog.FromContext(ctx)

	for _, ac := range actions {
		switch ac.Action {
		case "exec":
			ec := dexec.Config{
				User:       ac.User,
				Cmd:        ac.Cmd,
				Env:        ac.Env,
				WorkingDir: ac.WorkingDir,
				Stdout:     os.Stdout,
				Stderr:     os.Stderr,
			}

			if ac.Stdin != nil {
				ec.Stdin = strings.NewReader(*ac.Stdin)
			}

			if err := dexec.Exec(ctx, cli, containerID, ec); err != nil {
				return err
			}

		case "write", "append":
			var r io.Reader = strings.NewReader(ac.Contents)
			if ac.ContentsBase64 {
				r = base64.NewDecoder(base64.StdEncoding, r)
			}

			redir := ">"

			if ac.Action == "append" {
				redir = ">>"
			}

			ec := dexec.Config{
				User:       ac.User,
				Cmd:        []string{"sh", "-c", "cat " + redir + " " + ac.Filename},
				WorkingDir: ac.WorkingDir,
				Stdin:      r,
			}

			if err := dexec.Exec(ctx, cli, containerID, ec); err != nil {
				return err
			}

		default:
			logger.Warn("unknown action",
				zap.String("action", ac.Action),
			)
		}
	}

	return nil
}
