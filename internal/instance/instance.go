package instance

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/provision"
	"github.com/jnsgruk/dbx/internal/state"
	"github.com/jnsgruk/dbx/internal/tailscale"
)

// hostUID returns the uid of the user running dbx.
func hostUID() int { return os.Getuid() }

type Mount struct {
	Name   string
	Source string
	Dest   string
	File   bool // true for single files (VMs can't mount these, they get copied instead)
}

type CreateOpts struct {
	Image        string
	VM           bool
	CPU          string
	Mem          string
	Disk         string
	Extras       []string
	TailscaleKey string
}

func GenerateName(purpose, image string) (string, error) {
	base := imageBase(image)

	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random suffix: %w", err)
	}

	return fmt.Sprintf("%s-%s-%s", purpose, base, hex.EncodeToString(b)), nil
}

func imageBase(image string) string {
	if i := strings.LastIndex(image, ":"); i >= 0 {
		return image[i+1:]
	}
	return image
}

func DefaultMounts() []Mount {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	cwd, _ = filepath.EvalSymlinks(cwd)

	uhome := "/home/" + config.User
	return []Mount{
		{Name: "project", Source: cwd, Dest: filepath.Join(uhome, "project")},
		{Name: "scripts", Source: filepath.Join(home, "scripts"), Dest: filepath.Join(uhome, "scripts")},
		{Name: "claudedir", Source: filepath.Join(home, ".claude"), Dest: filepath.Join(uhome, ".claude")},
		{Name: "claudejson", Source: filepath.Join(home, ".claude.json"), Dest: filepath.Join(uhome, ".claude.json"), File: true},
		{Name: "ghdir", Source: filepath.Join(home, ".config", "gh"), Dest: filepath.Join(uhome, ".config", "gh")},
		{Name: "opencodecfg", Source: filepath.Join(home, ".config", "opencode"), Dest: filepath.Join(uhome, ".config", "opencode")},
		{Name: "opencodeauth", Source: filepath.Join(home, ".local", "share", "opencode", "auth.json"), Dest: filepath.Join(uhome, ".local", "share", "opencode", "auth.json"), File: true},
		{Name: "atuindir", Source: filepath.Join(home, ".local", "share", "atuin"), Dest: filepath.Join(uhome, ".local", "share", "atuin")},
	}
}

func pollUntil(timeout time.Duration, check func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func WaitForUser(project, name, user string, timeout time.Duration) error {
	ok := pollUntil(timeout, func() bool {
		_, err := lxc.Exec(project, name, "id", "-u", user)
		return err == nil
	})
	if !ok {
		return fmt.Errorf("timed out waiting for user %q in instance %q", user, name)
	}
	return nil
}

func WaitForNetwork(project, name string, timeout time.Duration) error {
	ok := pollUntil(timeout, func() bool {
		_, err := lxc.Exec(project, name, "getent", "hosts", "archive.ubuntu.com")
		return err == nil
	})
	if !ok {
		return fmt.Errorf("timed out waiting for network in instance %q", name)
	}
	return nil
}

func WaitForSnapd(project, name string, timeout time.Duration) error {
	ok := pollUntil(timeout, func() bool {
		_, err := lxc.Exec(project, name, "snap", "version")
		return err == nil
	})
	if !ok {
		return fmt.Errorf("timed out waiting for snapd in instance %q", name)
	}
	return nil
}

// CreateUser creates a new user and group inside the instance with the given
// name and uid/gid, plus passwordless sudo access. The default cloud-init
// user (ubuntu, uid 1000) is reassigned to a high uid first if it would
// conflict.
func CreateUser(project, name, user string, uid int) error {
	slog.Info("Creating user", "user", user, "uid", uid)

	// Move the cloud-init user's uid/gid out of the way.
	_, _ = lxc.Exec(project, name, "pkill", "-9", "-u", "ubuntu")
	if _, err := lxc.Exec(project, name, "usermod", "-u", config.RemapUID, "ubuntu"); err != nil {
		return fmt.Errorf("reassigning ubuntu uid: %w", err)
	}
	if _, err := lxc.Exec(project, name, "groupmod", "-g", config.RemapUID, "ubuntu"); err != nil {
		return fmt.Errorf("reassigning ubuntu gid: %w", err)
	}

	uidStr := fmt.Sprintf("%d", uid)
	if _, err := lxc.Exec(project, name, "groupadd", "-g", uidStr, user); err != nil {
		return fmt.Errorf("creating group: %w", err)
	}
	if _, err := lxc.Exec(project, name,
		"useradd", "-m", "-u", uidStr, "-g", uidStr, "-s", config.LoginShell, user,
	); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	// Grant passwordless sudo.
	sudoLine := user + " ALL=(ALL) NOPASSWD:ALL"
	if _, err := lxc.Exec(project, name,
		"bash", "-c", fmt.Sprintf("echo '%s' > /etc/sudoers.d/%s", sudoLine, user),
	); err != nil {
		return fmt.Errorf("configuring sudo: %w", err)
	}

	return nil
}

func InstallTerminfo(project, name string) error {
	infocmp := exec.Command("infocmp", "-x")
	output, err := infocmp.Output()
	if err != nil {
		return fmt.Errorf("running infocmp: %w", err)
	}

	_, err = lxc.ExecStdin(project, name, bytes.NewReader(output), "tic", "-x", "-")
	return err
}

// postStartSetup runs the common setup steps after an instance is started
// and the user exists. Used by both BuildBase and createFull.
func postStartSetup(project, name string) error {
	slog.Debug("Fixing home directory permissions", "name", name)
	chownArg := config.User + ":" + config.User
	if _, err := lxc.Exec(project, name, "sudo", "chown", chownArg, "/home/"+config.User); err != nil {
		return fmt.Errorf("fixing home directory permissions: %w", err)
	}

	slog.Debug("Installing ghostty terminfo", "name", name)
	if err := InstallTerminfo(project, name); err != nil {
		slog.Warn("Installing ghostty terminfo", "error", err)
	}

	return nil
}

// configureMounts adds mounts, saves state, starts the instance, and copies
// file mounts for VMs. waitUser is the username to poll for after start
// (config.User for base copies, "ubuntu" for fresh images before user creation).
func configureMounts(name string, opts CreateOpts, st state.State, cwd, waitUser string) error {
	if !opts.VM {
		if err := lxc.SetIDMap(name, hostUID()); err != nil {
			return fmt.Errorf("setting idmap: %w", err)
		}
	}

	mounts := DefaultMounts()
	for _, m := range mounts {
		if _, err := os.Stat(m.Source); os.IsNotExist(err) {
			slog.Warn("Skipping mount, source does not exist", "source", m.Source, "device", m.Name)
			continue
		}
		if m.File && opts.VM {
			slog.Debug("Deferring file mount for VM, will copy after start", "device", m.Name)
			continue
		}
		slog.Info("Adding mount", "device", m.Name, "source", m.Source, "dest", m.Dest)
		if err := lxc.DeviceAdd("", name, m.Name, m.Source, m.Dest); err != nil {
			return fmt.Errorf("adding mount %s: %w", m.Name, err)
		}
	}

	st[cwd] = name
	if err := state.Save(st); err != nil {
		slog.Warn("Saving state", "error", err)
	}

	slog.Info("Starting instance", "name", name)
	if err := lxc.Start("", name); err != nil {
		return fmt.Errorf("starting instance: %w", err)
	}

	slog.Info("Waiting for user", "name", name, "user", waitUser)
	if err := WaitForUser("", name, waitUser, 120*time.Second); err != nil {
		return err
	}

	if opts.VM {
		for _, m := range mounts {
			if !m.File {
				continue
			}
			if _, err := os.Stat(m.Source); os.IsNotExist(err) {
				continue
			}
			slog.Info("Copying file into VM", "source", m.Source, "dest", m.Dest)
			if err := lxc.FilePush(name, m.Source, m.Dest); err != nil {
				return fmt.Errorf("copying file %s: %w", m.Name, err)
			}
		}
	}

	return nil
}

// Create creates a new instance. If st is non-nil, it is used as the
// pre-loaded state to avoid a redundant LoadPruned call.
func Create(purpose string, opts CreateOpts, st state.State) (string, error) {
	cwd, _ := os.Getwd()
	cwd, _ = filepath.EvalSymlinks(cwd)
	if st == nil {
		st = state.LoadPruned()
	}
	if existing, ok := st[cwd]; ok {
		return existing, fmt.Errorf("directory already has instance %q (use 'dbx shell' to connect)", existing)
	}

	name, err := GenerateName(purpose, opts.Image)
	if err != nil {
		return "", err
	}

	// Try the fast path: copy from a base instance
	baseName, err := EnsureBase(opts)
	if err != nil {
		slog.Warn("Building base instance, falling back to full create", "error", err)
		return createFull(name, opts, st, cwd)
	}

	slog.Info("Creating instance from base", "name", name, "base", baseName)

	if err := lxc.CopyFromProject(baseProject, baseName, name); err != nil {
		return "", fmt.Errorf("copying from base: %w", err)
	}

	if opts.CPU != "" {
		if err := lxc.ConfigSet(name, "limits.cpu", opts.CPU); err != nil {
			return "", fmt.Errorf("setting cpu limit: %w", err)
		}
	}
	if opts.Mem != "" {
		if err := lxc.ConfigSet(name, "limits.memory", opts.Mem); err != nil {
			return "", fmt.Errorf("setting memory limit: %w", err)
		}
	}
	if opts.Disk != "" {
		if err := lxc.DeviceOverride(name, "root", "size="+opts.Disk); err != nil {
			return "", fmt.Errorf("setting disk size: %w", err)
		}
	}

	if err := configureMounts(name, opts, st, cwd, config.User); err != nil {
		return "", err
	}

	if len(opts.Extras) > 0 || opts.TailscaleKey != "" {
		slog.Info("Waiting for network", "name", name)
		if err := WaitForNetwork("", name, 60*time.Second); err != nil {
			return "", fmt.Errorf("waiting for network: %w", err)
		}

		slog.Info("Waiting for snapd", "name", name)
		if err := WaitForSnapd("", name, 60*time.Second); err != nil {
			return "", fmt.Errorf("waiting for snapd: %w", err)
		}
	}

	if len(opts.Extras) > 0 {
		slog.Info("Running extras provisioning", "name", name)
		if err := provision.RunExtras(name, config.User, opts.Extras); err != nil {
			return "", fmt.Errorf("running extras provisioning: %w", err)
		}
	}

	if opts.TailscaleKey != "" {
		slog.Info("Setting up Tailscale", "name", name)
		deviceID, err := provision.RunTailscale(name, config.User, opts.TailscaleKey)
		if err != nil {
			return "", fmt.Errorf("setting up tailscale: %w", err)
		}
		if err := tailscale.SaveDevice(name, deviceID); err != nil {
			slog.Warn("Saving Tailscale device state", "error", err)
		}
	}

	slog.Info("Instance ready", "name", name)
	return name, nil
}

// createFull is the fallback path when no base instance is available.
func createFull(name string, opts CreateOpts, st state.State, cwd string) (string, error) {
	slog.Info("Creating instance (full provisioning)", "name", name, "image", opts.Image, "vm", opts.VM)

	if err := lxc.Init("", name, opts.Image, opts.VM, opts.CPU, opts.Mem); err != nil {
		return "", fmt.Errorf("initializing instance: %w", err)
	}

	if opts.Disk != "" {
		if err := lxc.DeviceOverride(name, "root", "size="+opts.Disk); err != nil {
			return "", fmt.Errorf("setting disk size: %w", err)
		}
	}

	if err := configureMounts(name, opts, st, cwd, "ubuntu"); err != nil {
		return "", err
	}

	if err := CreateUser("", name, config.User, hostUID()); err != nil {
		return "", fmt.Errorf("creating user: %w", err)
	}

	if err := postStartSetup("", name); err != nil {
		return "", err
	}

	slog.Info("Waiting for network", "name", name)
	if err := WaitForNetwork("", name, 60*time.Second); err != nil {
		return "", fmt.Errorf("waiting for network: %w", err)
	}

	slog.Info("Waiting for snapd", "name", name)
	if err := WaitForSnapd("", name, 60*time.Second); err != nil {
		return "", fmt.Errorf("waiting for snapd: %w", err)
	}

	slog.Info("Running provisioning", "name", name)
	if err := provision.Run(name, config.User, opts.Extras); err != nil {
		return "", fmt.Errorf("running provisioning: %w", err)
	}

	if opts.VM {
		slog.Info("Running VM provisioning", "name", name)
		if err := provision.RunVM("", name, config.User); err != nil {
			return "", fmt.Errorf("running vm provisioning: %w", err)
		}
	}

	if opts.TailscaleKey != "" {
		slog.Info("Setting up Tailscale", "name", name)
		deviceID, err := provision.RunTailscale(name, config.User, opts.TailscaleKey)
		if err != nil {
			return "", fmt.Errorf("setting up tailscale: %w", err)
		}
		if err := tailscale.SaveDevice(name, deviceID); err != nil {
			slog.Warn("Saving Tailscale device state", "error", err)
		}
	}

	slog.Info("Instance ready", "name", name)
	return name, nil
}

func EnsureRunning(name string) error {
	info := lxc.GetInstanceInfo(name)
	if info.Status == "STOPPED" {
		slog.Info("Starting stopped instance", "name", name)
		if err := lxc.Start("", name); err != nil {
			return fmt.Errorf("starting instance: %w", err)
		}
		if err := WaitForUser("", name, config.User, 120*time.Second); err != nil {
			return err
		}
	}
	return nil
}
