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

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/state"
	"github.com/jnsgruk/dbx/internal/tailscale"
	"github.com/jnsgruk/dbx/internal/tools"
)

// hostUID returns the uid of the user running dbx.
func hostUID() int { return os.Getuid() }

type CreateOpts struct {
	Name         string // exact instance name; if empty, GenerateName is used
	Image        string
	VM           bool
	CPU          string
	Mem          string
	Disk         string
	Tools        []string // selected user tools
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

// toolContext builds a tools.Context for the given options and instance.
func toolContext(opts CreateOpts, project, instanceName, cwd string) tools.Context {
	home, _ := os.UserHomeDir()
	return tools.Context{
		HostHome:     home,
		InstanceHome: "/home/" + config.User,
		Cwd:          cwd,
		User:         config.User,
		GitHubUser:   config.GitHubUser,
		VM:           opts.VM,
		Project:      project,
		Instance:     instanceName,
	}
}

// collectMounts returns all mounts for the given selected tools.
func collectMounts(ctx tools.Context, selected []tools.Tool) []tools.Mount {
	var out []tools.Mount
	for _, t := range selected {
		out = append(out, t.Mounts(ctx)...)
	}
	return out
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
	if _, err := lxc.Exec(
		project, name,
		"useradd", "-m", "-u", uidStr, "-g", uidStr, "-s", config.LoginShell, user,
	); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	// Grant passwordless sudo.
	sudoLine := user + " ALL=(ALL) NOPASSWD:ALL"
	if _, err := lxc.Exec(
		project, name,
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

// attachMounts attaches disk mounts to the instance. File mounts are deferred
// for VMs and when the tool requests copy semantics.
func attachMounts(name string, opts CreateOpts, mounts []tools.Mount) error {
	if !opts.VM {
		if err := lxc.SetIDMap(name, hostUID()); err != nil {
			return fmt.Errorf("setting idmap: %w", err)
		}
	}

	for _, m := range mounts {
		if _, err := os.Stat(m.Source); os.IsNotExist(err) {
			slog.Warn("Skipping mount, source does not exist", "source", m.Source, "device", m.Name)
			continue
		}
		if m.File && (opts.VM || m.Copy) {
			slog.Debug("Deferring file mount, will copy after start", "device", m.Name)
			continue
		}
		// Remove any device inherited from the base instance copy so the
		// add below does not fail with "device already exists".
		_ = lxc.DeviceRemove("", name, m.Name)
		slog.Info("Adding mount", "device", m.Name, "source", m.Source, "dest", m.Dest)
		if err := lxc.DeviceAdd("", name, m.Name, m.Source, m.Dest); err != nil {
			return fmt.Errorf("adding mount %s: %w", m.Name, err)
		}
	}
	return nil
}

// startAndWaitForUser starts the instance and blocks until the given user is
// resolvable inside it.
func startAndWaitForUser(name, waitUser string) error {
	slog.Info("Starting instance", "name", name)
	if err := lxc.Start("", name); err != nil {
		return fmt.Errorf("starting instance: %w", err)
	}

	slog.Info("Waiting for user", "name", name, "user", waitUser)
	return WaitForUser("", name, waitUser, UserWaitTimeout)
}

// finalizeFileMounts fixes parent-dir ownership for file mounts and copies
// files that cannot or should not be attached as disk devices.
func finalizeFileMounts(name string, opts CreateOpts, mounts []tools.Mount) error {
	chownArg := fmt.Sprintf("%d:%d", hostUID(), hostUID())

	// Fix ownership of parent directories created by LXD for file mounts.
	// LXD creates these as root, which prevents the user from writing siblings.
	seen := map[string]bool{}
	for _, m := range mounts {
		if !m.File {
			continue
		}
		if _, err := os.Stat(m.Source); os.IsNotExist(err) {
			continue
		}
		parent := filepath.Dir(m.Dest)
		if seen[parent] {
			continue
		}
		seen[parent] = true
		if _, err := lxc.Exec("", name, "chown", chownArg, parent); err != nil {
			slog.Warn("Fixing file mount parent ownership", "path", parent, "error", err)
		}
	}

	for _, m := range mounts {
		if !m.File || (!opts.VM && !m.Copy) {
			continue
		}
		if _, err := os.Stat(m.Source); os.IsNotExist(err) {
			continue
		}
		slog.Info("Copying file into instance", "source", m.Source, "dest", m.Dest)
		if err := lxc.FilePush(name, m.Source, m.Dest); err != nil {
			return fmt.Errorf("copying file %s: %w", m.Name, err)
		}
		parent := filepath.Dir(m.Dest)
		if _, err := lxc.Exec("", name, "chown", chownArg, parent); err != nil {
			return fmt.Errorf("setting ownership on %s parent: %w", m.Name, err)
		}
		if _, err := lxc.Exec("", name, "chown", chownArg, m.Dest); err != nil {
			return fmt.Errorf("setting ownership on %s: %w", m.Name, err)
		}
		if m.Mode != "" {
			if _, err := lxc.Exec("", name, "chmod", m.Mode, m.Dest); err != nil {
				return fmt.Errorf("setting permissions on %s: %w", m.Name, err)
			}
		}
	}

	return nil
}

// recordInstance appends the instance to the state entry for cwd and persists.
// Save failures are logged but not fatal.
func recordInstance(st state.State, cwd, name string) {
	st[cwd] = append(st[cwd], name)
	if err := state.Save(st); err != nil {
		slog.Warn("Saving state", "error", err)
	}
}

// Create creates a new instance. If st is non-nil, it is used as the
// pre-loaded state to avoid a redundant LoadPruned call.
func Create(purpose string, opts CreateOpts, st state.State) (string, error) {
	cwd, _ := os.Getwd()
	cwd, _ = filepath.EvalSymlinks(cwd)
	if st == nil {
		st, _ = state.LoadPruned()
	}

	var name string
	if opts.Name != "" {
		name = opts.Name
	} else {
		var err error
		name, err = GenerateName(purpose, opts.Image)
		if err != nil {
			return "", err
		}
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

	if !opts.VM {
		if err := lxc.ConfigSet(name, "security.nesting", "true"); err != nil {
			return "", fmt.Errorf("setting nesting: %w", err)
		}
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

	ctx := toolContext(opts, "", name, cwd)
	selected := tools.Select(ctx, opts.Tools)
	mounts := collectMounts(ctx, selected)

	if err := attachMounts(name, opts, mounts); err != nil {
		return "", err
	}
	recordInstance(st, cwd, name)
	if err := startAndWaitForUser(name, config.User); err != nil {
		return "", err
	}
	if err := finalizeFileMounts(name, opts, mounts); err != nil {
		return "", err
	}

	// Only the opt-in tools run here; Always tools are already baked into the
	// base image. Authentication for Authenticator tools always runs when a
	// credential is supplied.
	var installList []tools.Tool
	for _, t := range selected {
		if a, ok := t.(tools.Always); ok && a.Always(ctx) {
			continue
		}
		installList = append(installList, t)
	}

	return finalizeInstance(ctx, name, opts, installList)
}

// finalizeInstance runs the post-start pipeline shared by the base-copy and
// full-provisioning paths: wait for network/snapd (if needed), install tools,
// authenticate tailscale (if requested), and wait for shell readiness.
func finalizeInstance(ctx tools.Context, name string, opts CreateOpts, installList []tools.Tool) (string, error) {
	if len(installList) > 0 || opts.TailscaleKey != "" {
		slog.Info("Waiting for network", "name", name)
		if err := WaitForNetwork("", name, NetworkWaitTimeout); err != nil {
			return "", fmt.Errorf("waiting for network: %w", err)
		}

		slog.Info("Waiting for snapd", "name", name)
		if err := WaitForSnapd("", name, SnapdWaitTimeout); err != nil {
			return "", fmt.Errorf("waiting for snapd: %w", err)
		}
	}

	if err := tools.Install(ctx, installList); err != nil {
		return "", err
	}

	if opts.TailscaleKey != "" {
		if err := runTailscaleAuth(ctx, name, opts.TailscaleKey); err != nil {
			return "", err
		}
	}

	if err := waitForShellReady("", name, config.User, ShellWaitTimeout); err != nil {
		return "", fmt.Errorf("waiting for shell readiness: %w", err)
	}

	slog.Info("Instance ready", "name", name)
	return name, nil
}

// runTailscaleAuth invokes the tailscale tool's Authenticator and persists the
// resulting device ID.
func runTailscaleAuth(ctx tools.Context, name, key string) error {
	t, ok := tools.Get("tailscale")
	if !ok {
		return fmt.Errorf("tailscale tool not registered")
	}
	auth, ok := t.(tools.Authenticator)
	if !ok {
		return fmt.Errorf("tailscale tool does not implement Authenticator")
	}
	slog.Info("Setting up Tailscale", "name", name)
	deviceID, err := auth.Authenticate(ctx, key)
	if err != nil {
		return err
	}
	if err := tailscale.SaveDevice(name, deviceID); err != nil {
		slog.Warn("Saving Tailscale device state", "error", err)
	}
	return nil
}

// createFull is the fallback path when no base instance is available.
func createFull(name string, opts CreateOpts, st state.State, cwd string) (string, error) {
	slog.Info("Creating instance (full provisioning)", "name", name, "image", opts.Image, "vm", opts.VM)

	if err := lxc.Init("", name, opts.Image, opts.VM, opts.CPU, opts.Mem, opts.Disk); err != nil {
		return "", fmt.Errorf("initializing instance: %w", err)
	}

	ctx := toolContext(opts, "", name, cwd)
	selected := tools.Select(ctx, opts.Tools)
	mounts := collectMounts(ctx, selected)

	if err := attachMounts(name, opts, mounts); err != nil {
		return "", err
	}
	recordInstance(st, cwd, name)
	if err := startAndWaitForUser(name, "ubuntu"); err != nil {
		return "", err
	}
	slog.Info("Waiting for cloud-init", "name", name)
	if err := WaitForCloudInit("", name, CloudInitWaitTimeout); err != nil {
		return "", err
	}
	if err := finalizeFileMounts(name, opts, mounts); err != nil {
		return "", err
	}

	if err := CreateUser("", name, config.User, hostUID()); err != nil {
		return "", err
	}

	if err := postStartSetup("", name); err != nil {
		return "", err
	}

	return finalizeInstance(ctx, name, opts, selected)
}

func EnsureRunning(name string) error {
	info := lxc.GetInstanceInfo(name)
	if info.Status == "STOPPED" {
		slog.Info("Starting stopped instance", "name", name)
		if err := lxc.Start("", name); err != nil {
			return fmt.Errorf("starting instance: %w", err)
		}
		if err := WaitForUser("", name, config.User, UserWaitTimeout); err != nil {
			return err
		}
		if err := waitForShellReady("", name, config.User, ShellWaitTimeout); err != nil {
			return fmt.Errorf("waiting for shell readiness: %w", err)
		}
	}
	return nil
}
