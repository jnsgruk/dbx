package tools

import "path/filepath"

type codex struct{}

func (codex) Name() string { return "codex" }
func (codex) Description() string {
	return "Codex CLI (via mise) with config/auth mounts"
}

func (codex) Mounts(ctx Context) []Mount {
	return []Mount{
		{
			Name:   "codexauth",
			Source: filepath.Join(ctx.HostHome, ".codex", "auth.json"),
			Dest:   filepath.Join(ctx.InstanceHome, ".codex", "auth.json"),
			File:   true,
			Mode:   "600",
		},
		{
			Name:   "codexconfig",
			Source: filepath.Join(ctx.HostHome, ".codex", "config.toml"),
			Dest:   filepath.Join(ctx.InstanceHome, ".codex", "config.toml"),
			File:   true,
			Copy:   true,
			Mode:   "600",
		},
	}
}

func (codex) InstallScript(Context) string {
	return `mise use -g nodejs
mise use -g npm:@openai/codex`
}

func init() { Register(codex{}) }
