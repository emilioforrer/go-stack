package main

import (
	"errors"
	"os"
)

var osExit = os.Exit

func main() {
	initCommands(rootCmd)

	if err := Execute(); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			osExit(exitErr.Code)
			return
		}
		osExit(1)
	}
}
