package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jnsgruk/dbx/internal/lxc"
)

// tailscaleTool installs the tailscale apt package. It is Always selected so
// every instance has the binary available; Authenticate only runs when the
// user explicitly provides an auth key.
type tailscaleTool struct{}

func (tailscaleTool) Name() string { return "tailscale" }
func (tailscaleTool) Description() string {
	return "Tailscale VPN client (install; auth via --tailscale)"
}
func (tailscaleTool) Always(Context) bool    { return true }
func (tailscaleTool) Hidden() bool           { return true }
func (tailscaleTool) Mounts(Context) []Mount { return nil }

func (tailscaleTool) InstallScript(Context) string {
	return strings.Join([]string{
		`export DEBIAN_FRONTEND=noninteractive`,
		`curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/jammy.noarmor.gpg | sudo tee /usr/share/keyrings/tailscale-archive-keyring.gpg >/dev/null`,
		`curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/jammy.tailscale-keyring.list | sudo tee /etc/apt/sources.list.d/tailscale.list`,
		`sudo apt-get update && sudo apt-get install -y tailscale`,
	}, "\n")
}

// tailscaleStatus mirrors the subset of `tailscale status --json` we need.
type tailscaleStatus struct {
	Self struct {
		ID string `json:"ID"`
	} `json:"Self"`
}

func (tailscaleTool) Authenticate(ctx Context, key string) (string, error) {
	authCmd := fmt.Sprintf("sudo tailscale up --auth-key=%s --ssh --operator=%s", key, ctx.User)
	slog.Info("Authenticating Tailscale")
	if err := lxc.ExecScript(ctx.Project, ctx.Instance, ctx.User, authCmd); err != nil {
		return "", fmt.Errorf("authenticating tailscale: %w", err)
	}

	slog.Info("Fetching Tailscale device ID")
	output, err := lxc.Exec(ctx.Project, ctx.Instance, "tailscale", "status", "--json")
	if err != nil {
		return "", fmt.Errorf("fetching tailscale device id: %w", err)
	}

	var status tailscaleStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return "", fmt.Errorf("parsing tailscale status: %w", err)
	}
	return status.Self.ID, nil
}

func init() { Register(tailscaleTool{}) }
