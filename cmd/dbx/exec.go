package main

import (
	"errors"

	"github.com/jnsgruk/dbx/internal/config"
	"github.com/jnsgruk/dbx/internal/instance"
	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "exec [name] -- <cmd...>",
		Short:              "Run a command inside an instance",
		DisableFlagParsing: false,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dash := cmd.ArgsLenAtDash()
			var posArgs, cmdArgs []string
			if dash >= 0 {
				posArgs = args[:dash]
				cmdArgs = args[dash:]
			} else {
				cmdArgs = args
			}
			if len(cmdArgs) == 0 {
				return errors.New("no command specified (use -- <cmd>)")
			}
			name, cwd, err := resolveInstance(posArgs)
			if err != nil {
				return err
			}
			if err := instance.EnsureRunning(name); err != nil {
				return err
			}
			return lxc.ExecInteractive(name, config.User, projectDir(cwd), cmdArgs)
		},
	}
}
