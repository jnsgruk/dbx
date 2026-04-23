package main

import (
	"log/slog"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/instance"
	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/state"
	"github.com/jnsgruk/dbx/internal/tailscale"
	"github.com/spf13/cobra"
)

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell [name]",
		Short: "Open an interactive shell in an instance",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, cwd, err := resolveInstance(args)
			if err != nil {
				return err
			}
			if err := instance.EnsureRunning(name); err != nil {
				return err
			}
			return lxc.Shell(name, config.User, projectDir(cwd))
		},
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop an instance",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _, err := resolveInstance(args)
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
			name, _, err := resolveInstance(args)
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
