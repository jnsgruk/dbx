package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// base provides the core provisioning baked into every instance: runs
// probuntu's provision-headless, installs ssh/ssh-import-id, imports GitHub
// keys, tweaks starship, and enables IPv4/IPv6 forwarding. Also contributes
// the universal mounts (project dir and ~/scripts).
type base struct{}

func (base) Name() string        { return "base" }
func (base) Description() string { return "Core provisioning (probuntu, ssh, starship, sysctl)" }
func (base) Always(Context) bool { return true }
func (base) Hidden() bool        { return true }

func (base) Mounts(ctx Context) []Mount {
	return []Mount{
		{
			Name:   "project",
			Source: ctx.Cwd,
			Dest:   filepath.Join(ctx.InstanceHome, filepath.Base(ctx.Cwd)),
		},
		{
			Name:   "scripts",
			Source: filepath.Join(ctx.HostHome, "scripts"),
			Dest:   filepath.Join(ctx.InstanceHome, "scripts"),
			Build:  true, // needed during base image build for provision-headless
		},
		{
			Name:   "ghdir",
			Source: filepath.Join(ctx.HostHome, ".config", "gh"),
			Dest:   filepath.Join(ctx.InstanceHome, ".config", "gh"),
		},
		{
			Name:   "atuindir",
			Source: filepath.Join(ctx.HostHome, ".local", "share", "atuin"),
			Dest:   filepath.Join(ctx.InstanceHome, ".local", "share", "atuin"),
		},
	}
}

func (base) InstallScript(ctx Context) string {
	return strings.Join([]string{
		fmt.Sprintf("mkdir -p ~/.local ~/.config && sudo chown -R %[1]s:%[1]s ~/.local ~/.config", ctx.User),
		"source ~/scripts/probuntu/provision-headless",
		"sudo apt-get install -y ssh-import-id openssh-server",
		"ssh-import-id-gh " + ctx.GitHubUser,
		`printf '[hostname]\nssh_only = false\n' >> ~/.config/starship.toml`,
		`printf 'net.ipv4.ip_forward=1\nnet.ipv6.conf.all.forwarding=1\n' | sudo tee /etc/sysctl.d/99-k8s-forwarding.conf`,
		"sudo sysctl --system",
	}, "\n")
}

func init() { Register(base{}) }
