package tools

import "path/filepath"

type claude struct{}

func (claude) Name() string { return "claude" }
func (claude) Description() string {
	return "Claude Code CLI (claude-code via mise) with config/auth mounts"
}

func (claude) Mounts(ctx Context) []Mount {
	return []Mount{
		{
			Name:   "claudedir",
			Source: filepath.Join(ctx.HostHome, ".claude"),
			Dest:   filepath.Join(ctx.InstanceHome, ".claude"),
		},
		{
			Name:   "claudejson",
			Source: filepath.Join(ctx.HostHome, ".claude.json"),
			Dest:   filepath.Join(ctx.InstanceHome, ".claude.json"),
			File:   true,
		},
	}
}

func (claude) InstallScript(Context) string { return "mise use -g claude-code@latest" }

func init() { Register(claude{}) }
