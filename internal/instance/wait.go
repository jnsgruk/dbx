package instance

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/lxc"
)

// Polling parameters and default timeouts for the wait helpers. These are
// referenced by callers in instance creation and ensure-running flows so
// timeouts are controlled centrally.
const (
	pollInterval = 200 * time.Millisecond

	UserWaitTimeout    = 120 * time.Second
	NetworkWaitTimeout = 60 * time.Second
	SnapdWaitTimeout   = 60 * time.Second
	ShellWaitTimeout   = 60 * time.Second
)

// pollUntil polls check() until it returns true or timeout elapses.
func pollUntil(timeout time.Duration, check func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

// WaitForUser blocks until the given user is resolvable inside the instance.
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

// WaitForNetwork blocks until DNS resolution of archive.ubuntu.com succeeds.
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

// WaitForSnapd blocks until snapd is responsive inside the instance.
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

func shellReadyArgs(user string) []string {
	if config.Shell != "fish" {
		return nil
	}
	return []string{
		"sudo", "-u", user, "-i", "bash", "-lc",
		"command -v mise >/dev/null && mise --version >/dev/null",
	}
}

// waitForShellReady blocks until shell startup dependencies are available.
// Fish startup runs `mise activate fish`, which can race snap mounts on boot.
func waitForShellReady(project, name, user string, timeout time.Duration) error {
	args := shellReadyArgs(user)
	if len(args) == 0 {
		return nil
	}

	slog.Info("Waiting for shell readiness", "name", name)
	ok := pollUntil(timeout, func() bool {
		_, err := lxc.Exec(project, name, args...)
		return err == nil
	})
	if !ok {
		return fmt.Errorf("timed out waiting for shell readiness in instance %q", name)
	}
	return nil
}
