package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type errorWriter struct {
	failOnCount int
	calls       int
}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	e.calls++
	if e.calls == e.failOnCount {
		return 0, errors.New("write error")
	}
	return len(p), nil
}

func TestVersionCmd(t *testing.T) {
	cmd := newVersionCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "app dev (commit: unknown, built: unknown)") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestVersionCmdError1(t *testing.T) {
	cmd := newVersionCmd()
	cmd.SetOut(&errorWriter{failOnCount: 1})
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "write error" {
		t.Errorf("expected 'write error', got %v", err)
	}
}

func TestVersionCmdError2(t *testing.T) {
	cmd := newVersionCmd()
	cmd.SetOut(&errorWriter{failOnCount: 2})
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "write error" {
		t.Errorf("expected 'write error', got %v", err)
	}
}
