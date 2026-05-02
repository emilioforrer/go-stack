package main

import (
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestMainSuccess(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	oldExit := osExit
	defer func() { osExit = oldExit }()

	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
	}

	// Make Execute succeed (e.g. asking for help)
	os.Args = []string{appName, "--help"}
	main()

	if exitCalled {
		t.Errorf("expected no exit on success")
	}
}

func TestMainFailure(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	oldExit := osExit
	defer func() { osExit = oldExit }()

	exitCode := -1
	osExit = func(code int) {
		exitCode = code
	}

	// Unknown command will cause Execute to fail
	os.Args = []string{appName, "unknown-command-that-fails"}
	main()

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestMainExitError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	oldExit := osExit
	defer func() { osExit = oldExit }()

	exitCode := -1
	osExit = func(code int) {
		exitCode = code
	}

	cmd := &cobra.Command{
		Use: "dummy-exit-error",
		RunE: func(cmd *cobra.Command, args []string) error {
			return &ExitError{Code: 42, Err: errors.New("test")}
		},
	}
	rootCmd.AddCommand(cmd)
	// We need to ensure we don't pollute the commands for other tests
	// Cobra doesn't have RemoveCommand, so we just let it be, or reset rootCmd.
	// We don't care much for tests.

	os.Args = []string{appName, "dummy-exit-error"}
	main()

	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
}
