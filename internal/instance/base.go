package instance

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/provision"
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

	// Mount the scripts directory so provisioning scripts are available
	home, _ := os.UserHomeDir()
	scriptsSource := filepath.Join(home, "scripts")
	if _, err := os.Stat(scriptsSource); err == nil {
		slog.Debug("Mounting scripts for base build", "source", scriptsSource)
		if err := lxc.DeviceAdd(baseProject, baseName, "scripts", scriptsSource, "/home/"+config.User+"/scripts"); err != nil {
			return fmt.Errorf("mounting scripts for base build: %w", err)
		}
	}

	if err := lxc.Start(baseProject, baseName); err != nil {
		return fmt.Errorf("starting base instance: %w", err)
	}

	slog.Info("Waiting for cloud-init user", "name", baseName)
	if err := WaitForUser(baseProject, baseName, "ubuntu", 120*time.Second); err != nil {
		return err
	}

	if err := CreateUser(baseProject, baseName, config.User, hostUID()); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	if err := postStartSetup(baseProject, baseName); err != nil {
		return err
	}

	slog.Info("Waiting for network", "name", baseName)
	if err := WaitForNetwork(baseProject, baseName, 60*time.Second); err != nil {
		return fmt.Errorf("waiting for network: %w", err)
	}

	slog.Info("Running base provisioning", "name", baseName)
	if err := provision.RunBase(baseProject, baseName, config.User); err != nil {
		_ = lxc.Delete(baseProject, baseName)
		return fmt.Errorf("running base provisioning: %w", err)
	}

	if opts.VM {
		slog.Info("Running VM provisioning", "name", baseName)
		if err := provision.RunVM(baseProject, baseName, config.User); err != nil {
			_ = lxc.Delete(baseProject, baseName)
			return fmt.Errorf("running vm provisioning: %w", err)
		}
	}

	// Remove the scripts mount so the base is clean for copying
	_ = lxc.DeviceRemove(baseProject, baseName, "scripts")

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
