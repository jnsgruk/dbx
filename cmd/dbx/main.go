package main

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/instance"
	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/provision"
	"github.com/jnsgruk/dbx/internal/state"
	"github.com/jnsgruk/dbx/internal/tailscale"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	version = "dev"
	commit  = "dev"
	date    string
)

var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func setupLogger(level string) {
	logLevel := new(slog.LevelVar)
	logLevel.Set(logLevels[strings.ToLower(level)])
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02T15:04:05"))
			}
			return a
		},
	})
	slog.SetDefault(slog.New(h))
}

type createFlags struct {
	vm        bool
	image     string
	cpu       string
	mem       string
	disk      string
	extras    string
	tailscale bool
}

func (f *createFlags) register(fs *pflag.FlagSet) {
	fs.BoolVar(&f.vm, "vm", false, "create a virtual machine instead of a container")
	fs.StringVarP(&f.image, "image", "i", config.Image, "base image")
	fs.StringVarP(&f.cpu, "cpu", "c", "", "CPU limit (e.g. 4)")
	fs.StringVarP(&f.mem, "mem", "m", "", "memory limit (e.g. 8GiB)")
	fs.StringVarP(&f.disk, "disk", "d", "", "disk size (e.g. 50GiB)")
	fs.StringVarP(&f.extras, "extras", "e", "", "comma-separated extras: "+strings.Join(provision.ListExtras(), ", "))
	fs.BoolVar(&f.tailscale, "tailscale", false, "install and authenticate tailscale")
}

func (f *createFlags) applyVMDefaults(cmd *cobra.Command) {
	if !f.vm {
		return
	}
	if !cmd.Flags().Changed("cpu") {
		f.cpu = "16"
	}
	if !cmd.Flags().Changed("mem") {
		f.mem = "16GiB"
	}
	if !cmd.Flags().Changed("disk") {
		f.disk = "100GiB"
	}
}

func (f *createFlags) opts() (instance.CreateOpts, error) {
	var extras []string
	if f.extras != "" {
		extras = strings.Split(f.extras, ",")
		available := provision.ListExtras()
		for _, e := range extras {
			if !slices.Contains(available, e) {
				return instance.CreateOpts{}, fmt.Errorf("unknown extra %q (available: %s)", e, strings.Join(available, ", "))
			}
		}
	}
	return instance.CreateOpts{
		Image:  f.image,
		VM:     f.vm,
		CPU:    f.cpu,
		Mem:    f.mem,
		Disk:   f.disk,
		Extras: extras,
	}, nil
}

func resolveInstance(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	st := state.LoadPruned()
	cwd, _ := os.Getwd()
	cwd, _ = filepath.EvalSymlinks(cwd)
	name, ok := st[cwd]
	if !ok {
		return "", fmt.Errorf("no instance associated with %s", cwd)
	}
	return name, nil
}

func newRootCmd() *cobra.Command {
	var (
		logLevel string
		flags    createFlags
	)

	root := &cobra.Command{
		Use:     "dbx",
		Short:   "Create and manage LXD development containers and VMs",
		Version: buildVersion(version, commit, date),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setupLogger(logLevel)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			cwd, _ = filepath.EvalSymlinks(cwd)
			st := state.LoadPruned()

			if name, ok := st[cwd]; ok {
				if err := instance.EnsureRunning(name); err != nil {
					return err
				}
				return lxc.Shell(name, config.User)
			}

			flags.applyVMDefaults(cmd)
			opts, err := flags.opts()
			if err != nil {
				return err
			}

			if flags.tailscale {
				slog.Info("Fetching Tailscale auth key")
				key, err := tailscale.AuthKey()
				if err != nil {
					return fmt.Errorf("setting up tailscale: %w", err)
				}
				opts.TailscaleKey = key
			}

			name, err := instance.Create(filepath.Base(cwd), opts, st)
			if err != nil {
				return err
			}
			return lxc.Shell(name, config.User)
		},
	}

	root.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "log verbosity: debug, info, warn, error")
	flags.register(root.Flags())

	root.AddCommand(newCreateCmd())
	root.AddCommand(newShellCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newStopCmd())
	root.AddCommand(newRmCmd())
	root.AddCommand(newBaseCmd())

	return root
}

func newCreateCmd() *cobra.Command {
	var flags createFlags

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new LXD development instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.applyVMDefaults(cmd)
			opts, err := flags.opts()
			if err != nil {
				return err
			}

			if flags.tailscale {
				slog.Info("Fetching Tailscale auth key")
				key, err := tailscale.AuthKey()
				if err != nil {
					return fmt.Errorf("setting up tailscale: %w", err)
				}
				opts.TailscaleKey = key
			}

			_, err = instance.Create(args[0], opts, nil)
			return err
		},
	}

	flags.register(cmd.Flags())
	return cmd
}

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell [name]",
		Short: "Open an interactive shell in an instance",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := resolveInstance(args)
			if err != nil {
				return err
			}
			if err := instance.EnsureRunning(name); err != nil {
				return err
			}
			return lxc.Shell(name, config.User)
		},
	}
}

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tracked instances",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := state.LoadPruned()

			if len(st) == 0 {
				fmt.Println("No tracked instances.")
				return nil
			}

			allInfo, err := lxc.ListInstanceInfo()
			if err != nil {
				return fmt.Errorf("listing instances: %w", err)
			}

			type entry struct {
				name string
				dir  string
				info lxc.InstanceInfo
			}

			home, _ := os.UserHomeDir()
			entries := make([]entry, 0, len(st))
			for dir, name := range st {
				display := dir
				if home != "" && strings.HasPrefix(dir, home) {
					display = "~" + dir[len(home):]
				}
				entries = append(entries, entry{name: name, dir: display, info: allInfo[name]})
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].info.CreatedAt.After(entries[j].info.CreatedAt)
			})

			headers := [5]string{"NAME", "STATUS", "IPV4", "DIRECTORY", "CREATED"}
			widths := [5]int{len(headers[0]), len(headers[1]), len(headers[2]), len(headers[3]), len(headers[4])}

			type row [5]string
			rows := make([]row, len(entries))
			for i, e := range entries {
				rows[i] = row{e.name, e.info.Status, e.info.IPv4, e.dir, timeAgo(e.info.CreatedAt)}
				for c := range widths {
					if len(rows[i][c]) > widths[c] {
						widths[c] = len(rows[i][c])
					}
				}
			}

			fmtStr := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%s\n",
				widths[0], widths[1], widths[2], widths[3])
			fmt.Printf(fmtStr, headers[0], headers[1], headers[2], headers[3], headers[4])
			for _, r := range rows {
				fmt.Printf(fmtStr, r[0], r[1], r[2], r[3], r[4])
			}
			return nil
		},
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop an instance",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := resolveInstance(args)
			if err != nil {
				return err
			}
			slog.Info("Stopping instance", "name", name)
			return lxc.Stop("", name)
		},
	}
}

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm [name]",
		Aliases: []string{"delete"},
		Short:   "Force-delete an instance",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := resolveInstance(args)
			if err != nil {
				return err
			}
			if err := tailscale.RemoveDeviceByInstance(name); err != nil {
				slog.Warn("Removing Tailscale device", "error", err)
			}

			slog.Info("Deleting instance", "name", name)
			if err := lxc.ForceDelete(name); err != nil {
				return err
			}

			st := state.Load()
			state.RemoveByName(st, name)
			return state.Save(st)
		},
	}
}

func newBaseCmd() *cobra.Command {
	base := &cobra.Command{
		Use:   "base",
		Short: "Manage base instances used for fast creation",
	}

	base.AddCommand(newBaseLsCmd())
	base.AddCommand(newBaseRmCmd())

	return base
}

func newBaseLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List base instances",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			bases, err := instance.ListBases()
			if err != nil {
				return err
			}
			if len(bases) == 0 {
				fmt.Println("No base instances.")
				return nil
			}
			for _, name := range bases {
				fmt.Println(name)
			}
			return nil
		},
	}
}

func newBaseRmCmd() *cobra.Command {
	var (
		vm    bool
		image string
	)

	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a base instance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return instance.RemoveBase(image, vm)
		},
	}

	cmd.Flags().BoolVar(&vm, "vm", false, "remove the VM base (default: container)")
	cmd.Flags().StringVarP(&image, "image", "i", config.Image, "base image")

	return cmd
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		days := int(math.Round(d.Hours() / 24))
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(math.Round(d.Hours() / (24 * 7)))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(math.Round(d.Hours() / (24 * 30)))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}

func buildVersion(version, commit, date string) string {
	result := version
	if commit != "" {
		result = fmt.Sprintf("%s\ncommit: %s", result, commit)
	}
	if date != "" {
		result = fmt.Sprintf("%s\nbuilt at: %s", result, date)
	}
	return result
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
