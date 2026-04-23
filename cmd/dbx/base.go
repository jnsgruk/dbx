package main

import (
	"fmt"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/instance"
	"github.com/spf13/cobra"
)

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
