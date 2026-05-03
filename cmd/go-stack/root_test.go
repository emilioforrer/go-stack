// Package main implements tests for the go-stack CLI.
package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRootCmd(t *testing.T) {
	t.Parallel()

	t.Run("help output contains expected text", func(t *testing.T) {
		t.Parallel()
		cmd := newRootCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "go-stack") {
			t.Errorf("expected output to contain 'go-stack', got: %s", out)
		}
		if !strings.Contains(out, "new") {
			t.Errorf("expected output to contain 'new', got: %s", out)
		}
	})

	t.Run("unknown subcommand returns error", func(t *testing.T) {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"asdasd"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error for unknown subcommand")
		}
		out := buf.String()
		if !strings.Contains(out, "unknown command") && !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("expected error or output to mention 'unknown command', got error: %v, output: %s", err, out)
		}
	})

	t.Run("no args prints help", func(t *testing.T) {
		t.Parallel()
		cmd := newRootCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "go-stack") {
			t.Errorf("expected output to contain 'go-stack', got: %s", out)
		}
		if !strings.Contains(out, "new") {
			t.Errorf("expected output to contain 'new', got: %s", out)
		}
	})
}

func TestNewCmdRunE(t *testing.T) {
	// Not parallel because subtests mutate the package-level newProjectFunc variable.

	t.Run("returns error when newProjectFunc fails", func(t *testing.T) {
		origFunc := newProjectFunc
		newProjectFunc = func(string, string, string) error {
			return errors.New("project creation failed")
		}
		defer func() { newProjectFunc = origFunc }()

		cmd := newRootCmd()
		cmd.SetArgs([]string{"new", "/tmp/test"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "project creation failed") {
			t.Errorf("expected error to contain 'project creation failed', got: %v", err)
		}
	})

	t.Run("calls newProjectFunc with args", func(t *testing.T) {
		calledPath := ""
		calledName := ""
		calledModule := ""
		origFunc := newProjectFunc
		newProjectFunc = func(p, n, m string) error {
			calledPath = p
			calledName = n
			calledModule = m
			return nil
		}
		defer func() { newProjectFunc = origFunc }()

		cmd := newRootCmd()
		cmd.SetArgs([]string{"new", "/tmp/myapp", "--name", "myapp", "--module", "example.com/myapp"})
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calledPath != "/tmp/myapp" {
			t.Errorf("expected path '/tmp/myapp', got: %s", calledPath)
		}
		if calledName != "myapp" {
			t.Errorf("expected name 'myapp', got: %s", calledName)
		}
		if calledModule != "example.com/myapp" {
			t.Errorf("expected module 'example.com/myapp', got: %s", calledModule)
		}
	})

	t.Run("missing args shows error", func(t *testing.T) {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"new"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "accepts 1 arg") {
			t.Errorf("expected error to contain 'accepts 1 arg', got: %v", err)
		}
	})
}

func TestMainExecution(t *testing.T) {
	// Not parallel because subtests mutate package-level state (osExit, rootCmd, newProjectFunc).

	t.Run("exit code 0 on success", func(t *testing.T) {
		called := 0
		osExit = func(code int) {
			called = code
		}
		defer func() { osExit = os.Exit }()

		rootCmd = newRootCmd()
		rootCmd.SetArgs([]string{"--help"})
		main()
		if called != 0 {
			t.Errorf("expected exit code 0, got: %d", called)
		}
	})

	t.Run("exit code 1 on error", func(t *testing.T) {
		called := 0
		osExit = func(code int) {
			called = code
		}
		defer func() { osExit = os.Exit }()

		rootCmd = newRootCmd()
		rootCmd.SetArgs([]string{"new"})
		main()
		if called != 1 {
			t.Errorf("expected exit code 1, got: %d", called)
		}
	})

	t.Run("exitError exits with code", func(t *testing.T) {
		called := 0
		osExit = func(code int) {
			called = code
		}
		defer func() { osExit = os.Exit }()

		origFunc := newProjectFunc
		newProjectFunc = func(string, string, string) error {
			return &exitError{code: 42}
		}
		defer func() { newProjectFunc = origFunc }()

		rootCmd = newRootCmd()
		rootCmd.SetArgs([]string{"new", "/tmp/test"})
		main()
		if called != 42 {
			t.Errorf("expected exit code 42, got: %d", called)
		}
	})
}

func TestInitCommands(t *testing.T) {
	t.Parallel()

	t.Run("initCommands is a no-op", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		initCommands(cmd)
		if len(cmd.Commands()) != 0 {
			t.Errorf("expected no commands, got: %d", len(cmd.Commands()))
		}
	})

	t.Run("initCommands covers the function", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "coverage"}
		initCommands(cmd)
	})
}

func TestExitError(t *testing.T) {
	t.Parallel()

	t.Run("Error returns correct message", func(t *testing.T) {
		t.Parallel()
		e := &exitError{code: 5}
		if e.Error() != "exit code 5" {
			t.Errorf("expected 'exit code 5', got: %s", e.Error())
		}
	})
}

func TestExecute(t *testing.T) {
	// Not parallel because it mutates the package-level rootCmd.
	t.Run("returns error when rootCmd execution fails", func(t *testing.T) {
		rootCmd = newRootCmd()
		rootCmd.SetArgs([]string{"new"})
		err := Execute()
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("returns nil when rootCmd execution succeeds", func(t *testing.T) {
		rootCmd = newRootCmd()
		rootCmd.SetArgs([]string{"--help"})
		err := Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
