package main

import (
	"errors"
	"testing"
)

func TestExitError(t *testing.T) {
	err := errors.New("test error")
	exitErr := &ExitError{Code: 1, Err: err}

	if exitErr.Error() != "test error" {
		t.Errorf("expected 'test error', got %s", exitErr.Error())
	}

	if exitErr.Unwrap() != err {
		t.Errorf("expected %v, got %v", err, exitErr.Unwrap())
	}
}
