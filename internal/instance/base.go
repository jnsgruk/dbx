package instance

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/tools"
)

const baseProject = "dbx"

func BaseName(image string, vm bool) string {
	kind := "container"
	if vm {
		kind = "vm"
	}
	return fmt.Sprintf("base-%s-%s", imageBase(image), kind)
}

func ensureProject() error {
	if err := lxc.ProjectCreate(baseProject); err != nil {
		if lxc.ProjectExists(baseProject) {
			return nil
		}
		return fmt.Errorf("creating project: %w", err)
	}
	slog.Info("Created LXD project for base instances", "project", baseProject)
	return nil
}

func BuildBase(baseName string, opts CreateOpts) error {
	if err := ensureProject(); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	slog.Info("Building base instance", "name", baseName, "image", opts.Image, "vm", opts.VM)

	if err := lxc.Init(baseProject, baseName, opts.Image, opts.VM, "", ""); err != nil {
		return fmt.Errorf("initializing base instance: %w", err)
	}

	// Base build context: no cwd-specific mounts, user not yet created. Only
	// Build-marked mounts from Always tools are attached.
	home, _ := os.UserHomeDir()
	ctx := tools.Context{
		HostHome:     home,
		InstanceHome: "/home/" + config.User,
		User:         config.User,
		GitHubUser:   config.GitHubUser,
		VM:           opts.VM,
		Project:      baseProject,
		Instance:     baseName,
	}
	selected := tools.SelectAlways(ctx)

	var buildMounts []tools.Mount
	for _, t := range selected {
		for _, m := range t.Mounts(ctx) {
			if m.Build {
				buildMounts = append(buildMounts, m)
			}
		}
	}

	for _, m := range buildMounts {
		if _, err := os.Stat(m.Source); err != nil {
			slog.Warn("Skipping build mount, source missing", "source", m.Source, "device", m.Name)
			continue
		}
		slog.Debug("Mounting for base build", "source", m.Source, "device", m.Name)
		if err := lxc.DeviceAdd(baseProject, baseName, m.Name, m.Source, filepath.Clean(m.Dest)); err != nil {
			return fmt.Errorf("mounting %s for base build: %w", m.Name, err)
		}
	}

	if err := lxc.Start(baseProject, baseName); err != nil {
		return fmt.Errorf("starting base instance: %w", err)
	}

	slog.Info("Waiting for cloud-init user", "name", baseName)
	if err := WaitForUser(baseProject, baseName, "ubuntu", UserWaitTimeout); err != nil {
		return err
	}

	if err := CreateUser(baseProject, baseName, config.User, hostUID()); err != nil {
		return err
	}

	if err := postStartSetup(baseProject, baseName); err != nil {
		return err
	}

	slog.Info("Waiting for network", "name", baseName)
	if err := WaitForNetwork(baseProject, baseName, NetworkWaitTimeout); err != nil {
		return fmt.Errorf("waiting for network: %w", err)
	}

	slog.Info("Running base provisioning", "name", baseName)
	if err := tools.Install(ctx, selected); err != nil {
		_ = lxc.Delete(baseProject, baseName)
		return fmt.Errorf("running base provisioning: %w", err)
	}

	// Remove build-only mounts so the base is clean for copying.
	for _, m := range buildMounts {
		_ = lxc.DeviceRemove(baseProject, baseName, m.Name)
	}

	slog.Info("Stopping base instance", "name", baseName)
	if err := lxc.Stop(baseProject, baseName); err != nil {
		return fmt.Errorf("stopping base instance: %w", err)
	}

	slog.Info("Base instance ready", "name", baseName)
	return nil
}

func EnsureBase(opts CreateOpts) (string, error) {
	baseName := BaseName(opts.Image, opts.VM)

	if lxc.InstanceExistsInProject(baseProject, baseName) {
		slog.Debug("Using existing base instance", "name", baseName)
		return baseName, nil
	}

	if err := BuildBase(baseName, opts); err != nil {
		return "", err
	}
	return baseName, nil
}

func RemoveBase(image string, vm bool) error {
	baseName := BaseName(image, vm)
	slog.Info("Deleting base instance", "name", baseName)
	if err := lxc.Delete(baseProject, baseName); err != nil {
		return fmt.Errorf("deleting base instance %q: %w", baseName, err)
	}
	return nil
}

func ListBases() ([]string, error) {
	if !lxc.ProjectExists(baseProject) {
		return nil, nil
	}
	return lxc.ListInstancesInProject(baseProject)
}
