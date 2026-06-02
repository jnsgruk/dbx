package lxc

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"al.essio.dev/pkg/shellescape"
	"github.com/jnsgruk/dbx/internal/config"
)

func Run(args ...string) ([]byte, error) {
	return RunStdin(nil, args...)
}

func RunStdin(stdin io.Reader, args ...string) ([]byte, error) {
	commandString := "lxc " + strings.Join(args, " ")
	slog.Debug("Starting command", "command", commandString)

	start := time.Now()
	cmd := exec.Command("lxc", args...)
	cmd.Stdin = stdin
	output, err := cmd.CombinedOutput()

	slog.Debug("Finished command", "command", commandString, "elapsed", time.Since(start))

	if err != nil {
		return output, fmt.Errorf("%s: %w\n%s", commandString, err, string(output))
	}
	return output, nil
}

// projectArgs inserts --project after the subcommand (first arg).
// lxc requires: lxc <subcommand> --project <name> [args...]
func projectArgs(project string, args ...string) []string {
	if project == "" || len(args) == 0 {
		return args
	}
	result := make([]string, 0, len(args)+2)
	result = append(result, args[0], "--project", project)
	result = append(result, args[1:]...)
	return result
}

func Init(project, name, image string, vm bool, cpu, mem, disk string) error {
	args := projectArgs(project, "init", "-q", image, name)
	if vm {
		args = append(args, "--vm")
	}
	if cpu != "" {
		args = append(args, "-c", "limits.cpu="+cpu)
	}
	if mem != "" {
		args = append(args, "-c", "limits.memory="+mem)
	}
	if disk != "" {
		args = append(args, "-d", "root,size="+disk)
	}
	_, err := Run(args...)
	return err
}

// SetIDMap maps the host uid/gid to containerUID inside the container.
func SetIDMap(name string, containerUID int) error {
	uid := os.Getuid()
	gid := os.Getgid()
	idmap := fmt.Sprintf("uid %d %d\ngid %d %d", uid, containerUID, gid, containerUID)
	_, err := RunStdin(
		bytes.NewBufferString(idmap),
		"config", "set", "-q", name, "raw.idmap", "-",
	)
	return err
}

func ConfigSet(name, key, value string) error {
	_, err := Run("config", "set", name, key, value)
	return err
}

func DeviceAdd(project, name, device, source, path string) error {
	args := projectArgs(project, "config", "device", "add", "-q", name, device, "disk",
		"source="+source, "path="+path,
	)
	_, err := Run(args...)
	return err
}

func DeviceRemove(project, name, device string) error {
	args := projectArgs(project, "config", "device", "remove", "-q", name, device)
	_, err := Run(args...)
	return err
}

func DeviceOverride(name, device string, kv ...string) error {
	args := append([]string{"config", "device", "override", "-q", name, device}, kv...)
	_, err := Run(args...)
	if err == nil {
		return nil
	}
	// If the device already exists at the instance level (e.g. after a
	// cross-project copy), override fails. Fall back to device set.
	setArgs := append([]string{"config", "device", "set", "-q", name, device}, kv...)
	_, setErr := Run(setArgs...)
	return setErr
}

func Start(project, name string) error {
	_, err := Run(projectArgs(project, "start", name)...)
	return err
}

func Stop(project, name string) error {
	_, err := Run(projectArgs(project, "stop", name)...)
	return err
}

func Delete(project, name string) error {
	_, err := Run(projectArgs(project, "delete", name)...)
	return err
}

func ForceDelete(name string) error {
	_, err := Run("delete", "-f", name)
	return err
}

func Exec(project, name string, args ...string) ([]byte, error) {
	return ExecStdin(project, name, nil, args...)
}

func ExecStdin(project, name string, stdin io.Reader, args ...string) ([]byte, error) {
	cmdArgs := append(projectArgs(project, "exec", name, "--"), args...)
	return RunStdin(stdin, cmdArgs...)
}

func ExecScript(project, name, user, script string) error {
	_, err := Exec(project, name, "sudo", "-u", user, "bash", "-c", script)
	return err
}

func FilePush(name, source, dest string) error {
	_, err := Run("file", "push", "--create-dirs", source, name+dest)
	return err
}

func Shell(name, user, dir string) error {
	args := []string{"exec", name, "--", "sudo", "-u", user, "-i", config.Shell}
	if dir != "" {
		args = append(args, "-C", "cd "+dir+" 2>/dev/null")
	}
	cmd := exec.Command("lxc", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExecInteractive runs argv inside the instance as user, in dir, wiring
// stdio from the current process. The command runs via a login shell so
// that mise/PATH shims from ~/.profile are loaded.
func ExecInteractive(name, user, dir string, argv []string) error {
	script := shellescape.QuoteCommand(argv)
	if dir != "" {
		script = "cd " + shellescape.Quote(dir) + " 2>/dev/null; " + script
	}
	args := []string{"exec", name, "--", "sudo", "-u", user, "-i", config.Shell, "-c", script}
	cmd := exec.Command("lxc", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
