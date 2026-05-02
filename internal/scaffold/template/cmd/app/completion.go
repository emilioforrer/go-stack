package main

import (
	"github.com/spf13/cobra"
)

const (
	shellBash       = "bash"
	shellZsh        = "zsh"
	shellFish       = "fish"
	shellPowerShell = "powershell"
)

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion [" + shellBash + "|" + shellZsh + "|" + shellFish + "|" + shellPowerShell + "]",
		Short:     "Generate shell completion script",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{shellBash, shellZsh, shellFish, shellPowerShell},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case shellBash:
				return rootCmd.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case shellZsh:
				return rootCmd.GenZshCompletion(cmd.OutOrStdout())
			case shellFish:
				return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
			case shellPowerShell:
				return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}
			return nil
		},
	}
}
