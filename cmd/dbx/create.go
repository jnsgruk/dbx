package main

import (
	"fmt"
	"log/slog"

	"github.com/jnsgruk/dbx/internal/instance"
	"github.com/jnsgruk/dbx/internal/tailscale"
	"github.com/spf13/cobra"
)

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
					return fmt.Errorf("fetching tailscale auth key: %w", err)
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
