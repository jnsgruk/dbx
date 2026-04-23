package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/instance"
	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/state"
	"github.com/jnsgruk/dbx/internal/tailscale"
	"github.com/jnsgruk/dbx/internal/tools"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type createFlags struct {
	vm        bool
	image     string
	cpu       string
	mem       string
	disk      string
	tools     string
	tailscale bool
	name      string
	forceNew  bool
}

func (f *createFlags) register(fs *pflag.FlagSet) {
	fs.BoolVar(&f.vm, "vm", false, "create a virtual machine instead of a container")
	fs.StringVarP(&f.image, "image", "i", config.Image, "base image")
	fs.StringVarP(&f.cpu, "cpu", "c", "", "CPU limit (default 16 for VMs)")
	fs.StringVarP(&f.mem, "mem", "m", "", "memory limit (default 16GiB for VMs)")
	fs.StringVarP(&f.disk, "disk", "d", "", "disk size (default 100GiB for VMs)")
	fs.StringVarP(&f.tools, "tools", "t", "", "comma-separated tools: "+strings.Join(tools.Names(), ", "))
	fs.BoolVar(&f.tailscale, "tailscale", false, "authenticate tailscale for the instance")
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
	var selected []string
	if f.tools != "" {
		selected = strings.Split(f.tools, ",")
		if err := tools.Validate(selected); err != nil {
			return instance.CreateOpts{}, err
		}
	}
	return instance.CreateOpts{
		Image: f.image,
		VM:    f.vm,
		CPU:   f.cpu,
		Mem:   f.mem,
		Disk:  f.disk,
		Tools: selected,
	}, nil
}

// findInstance returns the name of an existing instance to connect to based on
// the instances associated with the current directory and the provided name
// filter. It returns an empty name when a new instance should be created.
func findInstance(names []string, name string) (string, error) {
	if name != "" {
		if slices.Contains(names, name) {
			return name, nil
		}
		return "", nil
	}
	if len(names) == 1 {
		return names[0], nil
	}
	return "", fmt.Errorf("directory has multiple instances: %s (use -n to select or --new to create)", strings.Join(names, ", "))
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
			st, live := state.LoadPruned()
			tailscale.PruneDevices(live)

			if !flags.forceNew && len(st[cwd]) > 0 {
				target, err := findInstance(st[cwd], flags.name)
				if err != nil {
					return err
				}
				if target != "" {
					if err := instance.EnsureRunning(target); err != nil {
						return err
					}
					return lxc.Shell(target, config.User, projectDir(cwd))
				}
			}

			flags.applyVMDefaults(cmd)
			opts, err := flags.opts()
			if err != nil {
				return err
			}

			if flags.name != "" {
				opts.Name = flags.name
			}

			if flags.tailscale {
				slog.Info("Fetching Tailscale auth key")
				key, err := tailscale.AuthKey()
				if err != nil {
					return fmt.Errorf("fetching tailscale auth key: %w", err)
				}
				opts.TailscaleKey = key
			}

			name, err := instance.Create(filepath.Base(cwd), opts, st)
			if err != nil {
				return err
			}
			return lxc.Shell(name, config.User, projectDir(cwd))
		},
	}

	root.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "log verbosity: debug, info, warn, error")
	flags.register(root.Flags())
	root.Flags().StringVarP(&flags.name, "name", "n", "", "instance hostname")
	root.Flags().BoolVar(&flags.forceNew, "new", false, "create a new instance even if one exists for this directory")

	root.AddCommand(newCreateCmd())
	root.AddCommand(newShellCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newStopCmd())
	root.AddCommand(newRmCmd())
	root.AddCommand(newBaseCmd())
	root.AddCommand(newToolsCmd())

	return root
}
