// Package main implements the go-stack CLI.
package main

import (
	"errors"
	"os"
)

var osExit = os.Exit

func main() {
	initCommands(rootCmd)

	if err := Execute(); err != nil {
		var exitErr *exitError
		if errors.As(err, &exitErr) {
			osExit(exitErr.code)
			return
		}
		osExit(1)
	}
}
