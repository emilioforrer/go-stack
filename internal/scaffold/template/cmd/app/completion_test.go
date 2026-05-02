package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionCmd(t *testing.T) {
	oldRootCmd := rootCmd
	defer func() { rootCmd = oldRootCmd }()

	rootCmd = &cobra.Command{Use: appName}

	// Add some dummy commands to make completion have something to do
	rootCmd.AddCommand(&cobra.Command{Use: "dummy"})

	tests := []struct {
		shell string
	}{
		{shellBash},
		{shellZsh},
		{shellFish},
		{shellPowerShell},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			cmd := newCompletionCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{tt.shell})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if buf.Len() == 0 {
				t.Errorf("expected completion script, got nothing")
			}
		})
	}
}

func TestCompletionCmdInvalid(t *testing.T) {
	cmd := newCompletionCmd()
	// Call RunE directly to bypass ValidArgs validation and hit the fallback `return nil`
	err := cmd.RunE(cmd, []string{"invalid"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}
