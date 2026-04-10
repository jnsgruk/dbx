package provision

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/lxc"
)

//go:embed scripts/extras/*
var extrasFS embed.FS

// baseCommands returns the default provisioning sequence run on every instance.
// Each entry is passed to bash -c as the given user. These must run
// sequentially because later commands depend on earlier ones.
func baseCommands(user string) []string {
	return []string{
		fmt.Sprintf("mkdir -p ~/.local ~/.config && sudo chown -R %[1]s:%[1]s ~/.local ~/.config", user),
		"source ~/scripts/probuntu/provision-headless",
	}
}

// postProvisionScript runs after the base commands as a single script to
// reduce subprocess round-trips. These commands are independent of each other.
var postProvisionScript = strings.Join([]string{
	"sudo apt-get install -y ssh-import-id openssh-server",
	"ssh-import-id-gh " + config.GitHubUser,
	`printf '[hostname]\nssh_only = false\n' >> ~/.config/starship.toml`,
}, "\n")

func ListExtras() []string {
	entries, err := extrasFS.ReadDir("scripts/extras")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, strings.TrimSuffix(e.Name(), ".sh"))
		}
	}
	return names
}

func getExtra(name string) (string, error) {
	data, err := extrasFS.ReadFile(path.Join("scripts/extras", name+".sh"))
	if err != nil {
		return "", fmt.Errorf("unknown extra %q (available: %s)", name, strings.Join(ListExtras(), ", "))
	}
	return string(data), nil
}

func RunBase(project, instanceName, user string) error {
	for _, cmd := range baseCommands(user) {
		slog.Info("Provisioning", "command", cmd)
		if err := lxc.ExecScript(project, instanceName, user, cmd); err != nil {
			return fmt.Errorf("running provisioning command %q: %w", cmd, err)
		}
	}

	slog.Info("Provisioning", "command", "post-provision script")
	if err := lxc.ExecScript(project, instanceName, user, postProvisionScript); err != nil {
		return fmt.Errorf("running post-provision script: %w", err)
	}

	return nil
}

func RunExtras(instanceName, user string, extras []string) error {
	for _, extra := range extras {
		content, err := getExtra(extra)
		if err != nil {
			return err
		}
		slog.Info("Running extras script", "extra", extra)
		script := "#!/usr/bin/env bash\nset -exo pipefail\n\n" + content
		if err := lxc.ExecScript("", instanceName, user, script); err != nil {
			return fmt.Errorf("running extras script %q: %w", extra, err)
		}
	}
	return nil
}

func Run(instanceName, user string, extras []string) error {
	if err := RunBase("", instanceName, user); err != nil {
		return err
	}
	return RunExtras(instanceName, user, extras)
}

// tailscaleCommands installs the Tailscale package from the official apt repo.
var tailscaleCommands = []string{
	`export DEBIAN_FRONTEND=noninteractive && curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/jammy.noarmor.gpg | sudo tee /usr/share/keyrings/tailscale-archive-keyring.gpg >/dev/null`,
	`curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/jammy.tailscale-keyring.list | sudo tee /etc/apt/sources.list.d/tailscale.list`,
	`sudo apt-get update && sudo apt-get install -y tailscale`,
}

// tailscaleStatus is the subset of `tailscale status --json` we need.
type tailscaleStatus struct {
	Self struct {
		ID string `json:"ID"`
	} `json:"Self"`
}

// RunTailscale installs tailscale and authenticates using the given auth key.
// It returns the device ID assigned by the tailnet.
func RunTailscale(instanceName, user, authKey string) (string, error) {
	for _, cmd := range tailscaleCommands {
		slog.Info("Provisioning Tailscale", "command", cmd)
		if err := lxc.ExecScript("", instanceName, user, cmd); err != nil {
			return "", fmt.Errorf("running tailscale provisioning: %w", err)
		}
	}

	authCmd := fmt.Sprintf("sudo tailscale up --auth-key=%s --ssh --operator=%s", authKey, user)
	slog.Info("Authenticating Tailscale")
	if err := lxc.ExecScript("", instanceName, user, authCmd); err != nil {
		return "", fmt.Errorf("authenticating tailscale: %w", err)
	}

	slog.Info("Fetching Tailscale device ID")
	output, err := lxc.Exec("", instanceName, "tailscale", "status", "--json")
	if err != nil {
		return "", fmt.Errorf("fetching tailscale device id: %w", err)
	}

	var status tailscaleStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return "", fmt.Errorf("parsing tailscale status: %w", err)
	}
	return status.Self.ID, nil
}
