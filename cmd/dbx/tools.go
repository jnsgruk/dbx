package main

import (
	"fmt"

	"github.com/jnsgruk/dbx/internal/tools"
	"github.com/spf13/cobra"
)

func newToolsCmd() *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "List available tools",
	}
	toolsCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List user-selectable tools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var nameWidth int
			for _, t := range tools.All() {
				if tools.IsHidden(t) {
					continue
				}
				if len(t.Name()) > nameWidth {
					nameWidth = len(t.Name())
				}
			}
			for _, t := range tools.All() {
				if tools.IsHidden(t) {
					continue
				}
				fmt.Printf("%-*s  %s\n", nameWidth, t.Name(), t.Description())
			}
			return nil
		},
	})
	return toolsCmd
}
