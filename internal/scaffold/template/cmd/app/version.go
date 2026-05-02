package main

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "app %s (commit: %s, built: %s)\n", version, commit, date); err != nil {
				return err
			}

			if info, ok := debug.ReadBuildInfo(); ok {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "go: %s\n", info.GoVersion); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
