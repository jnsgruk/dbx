package tools

import "path/filepath"

type opencode struct{}

func (opencode) Name() string        { return "opencode" }
func (opencode) Description() string { return "opencode CLI (via mise) with config/auth mounts" }

func (opencode) Mounts(ctx Context) []Mount {
	return []Mount{
		{
			Name:   "opencodecfg",
			Source: filepath.Join(ctx.HostHome, ".config", "opencode"),
			Dest:   filepath.Join(ctx.InstanceHome, ".config", "opencode"),
		},
		{
			Name:   "opencodeauth",
			Source: filepath.Join(ctx.HostHome, ".local", "share", "opencode", "auth.json"),
			Dest:   filepath.Join(ctx.InstanceHome, ".local", "share", "opencode", "auth.json"),
			File:   true,
			Mode:   "600",
		},
	}
}

func (opencode) InstallScript(Context) string {
	return `mise use -g opencode@latest

mkdir -p "$HOME/.config/opencode"
if [ ! -f "$HOME/.config/opencode/opencode.json" ]; then
	cat > "$HOME/.config/opencode/opencode.json" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "edit": "allow",
    "bash": "allow",
    "webfetch": "allow"
  }
}
EOF
fi
`
}

func init() { Register(opencode{}) }
