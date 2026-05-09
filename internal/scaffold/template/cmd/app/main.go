package main

import (
	"errors"
	"fmt"
	"os"
)

var osExit = os.Exit

func main() {
	initCommands(rootCmd)

	if err := Execute(); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(os.Stderr, exitErr.Err)
			osExit(exitErr.Code)
			return
		}
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
}
