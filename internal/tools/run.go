package tools

import (
	"fmt"
	"log/slog"

	"github.com/jnsgruk/dbx/internal/lxc"
)

// Install runs the install script of each tool, in order, as the target user.
// Empty scripts are skipped. Each tool's script is wrapped with a bash
// shebang + `set -exo pipefail` and executed as a single ExecScript call.
func Install(ctx Context, selected []Tool) error {
	for _, t := range selected {
		script := t.InstallScript(ctx)
		if script == "" {
			continue
		}
		slog.Info("Running tool install", "tool", t.Name(), "instance", ctx.Instance)
		wrapped := "#!/usr/bin/env bash\nset -exo pipefail\n\n" + script + "\n"
		if err := lxc.ExecScript(ctx.Project, ctx.Instance, ctx.User, wrapped); err != nil {
			return fmt.Errorf("installing tool %q: %w", t.Name(), err)
		}
	}
	return nil
}
