package buncmd

import (
	"testing"
)

func TestNewCommand(t *testing.T) {
	t.Parallel()

	root := NewCommand("bundb")
	if root == nil {
		t.Fatal("expected non-nil root command")
	}

	if root.Use != "bundb" {
		t.Fatalf("expected Use=bundb, got %q", root.Use)
	}

	commands := root.Commands()
	if len(commands) == 0 {
		t.Fatal("expected root to have subcommands")
	}

	var hasDB, hasMigrate bool
	for _, c := range commands {
		switch c.Use {
		case "db":
			hasDB = true
		case "migrate":
			hasMigrate = true
		}
	}

	if !hasDB {
		t.Error("expected db subcommand")
	}
	if !hasMigrate {
		t.Error("expected migrate subcommand")
	}
}

func TestNewDBCmd(t *testing.T) {
	t.Parallel()

	c := &Cmd{}
	cmd := c.newDBCmd()
	if cmd == nil {
		t.Fatal("expected non-nil db command")
	}

	commands := cmd.Commands()
	if len(commands) == 0 {
		t.Fatal("expected db to have subcommands")
	}

	var hasCreate, hasDrop, hasReset, hasTruncate bool
	for _, c := range commands {
		switch c.Use {
		case "create":
			hasCreate = true
		case "drop":
			hasDrop = true
		case "reset":
			hasReset = true
		case "truncate":
			hasTruncate = true
		}
	}

	if !hasCreate {
		t.Error("expected create subcommand")
	}
	if !hasDrop {
		t.Error("expected drop subcommand")
	}
	if !hasReset {
		t.Error("expected reset subcommand")
	}
	if !hasTruncate {
		t.Error("expected truncate subcommand")
	}
}

func TestNewMigrateCmd(t *testing.T) {
	t.Parallel()

	c := &Cmd{}
	cmd := c.newMigrateCmd()
	if cmd == nil {
		t.Fatal("expected non-nil migrate command")
	}

	commands := cmd.Commands()
	if len(commands) == 0 {
		t.Fatal("expected migrate to have subcommands")
	}

	var hasInit, hasUp, hasDown, hasStatus, hasCreate bool
	for _, c := range commands {
		switch c.Use {
		case "init":
			hasInit = true
		case "up":
			hasUp = true
		case "down":
			hasDown = true
		case "status":
			hasStatus = true
		case "create [name]":
			hasCreate = true
		}
	}

	if !hasInit {
		t.Error("expected init subcommand")
	}
	if !hasUp {
		t.Error("expected up subcommand")
	}
	if !hasDown {
		t.Error("expected down subcommand")
	}
	if !hasStatus {
		t.Error("expected status subcommand")
	}
	if !hasCreate {
		t.Error("expected create subcommand")
	}
}

func TestExecute_NoArgs(t *testing.T) {
	t.Parallel()

	cmd := NewCommand("bundb")
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
}
