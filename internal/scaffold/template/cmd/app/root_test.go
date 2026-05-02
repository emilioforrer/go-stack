package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitConfig(t *testing.T) {
	// Test default config
	cfgFile = ""
	err := initConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test specific config file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(tmpFile, []byte("log-level: "+strDebug+"\n"), 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfgFile = tmpFile
	err = initConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if viper.GetString("log-level") != strDebug {
		t.Errorf("expected log-level debug, got %s", viper.GetString("log-level"))
	}

	// Test non-existing specific config file
	cfgFile = filepath.Join(tmpDir, "does-not-exist.yaml")
	err = initConfig()
	if err == nil {
		t.Errorf("expected error for non-existing config file")
	}
}

func TestInitConfigHomeError(t *testing.T) {
	oldUserHomeDir := userHomeDir
	defer func() { userHomeDir = oldUserHomeDir }()

	userHomeDir = func() (string, error) {
		return "", errors.New("home error")
	}

	cfgFile = ""
	err := initConfig()
	if err == nil {
		t.Errorf("expected error due to home dir failure")
	}
}

func TestInitConfigReadError(t *testing.T) {
	// To cause viper.ReadInConfig to return an error other than ConfigFileNotFoundError
	// we can pass a malformed yaml file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "bad-config.yaml")
	err := os.WriteFile(tmpFile, []byte("invalid\n  yaml:\ncontent: -"), 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfgFile = tmpFile
	err = initConfig()
	if err == nil {
		t.Errorf("expected error due to bad config format")
	}
}

func TestInitCommands(t *testing.T) {
	initCommands(rootCmd)
	if !rootCmd.HasSubCommands() {
		t.Errorf("expected root command to have subcommands")
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		level string
	}{
		{"info"},
		{strDebug},
		{"warn"},
		{"error"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			viper.Set("log-level", tt.level)
			// Reset cfgFile so it uses viper settings that were set
			cfgFile = ""
			_ = initConfig()
		})
	}
}

func TestRootCmdPersistentPreRun(t *testing.T) {
	err := rootCmd.PersistentPreRunE(rootCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecute(t *testing.T) {
	// Just test that Execute runs rootCmd.Execute
	// We can't really intercept it without mocking rootCmd,
	// but running it covers the line.
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{appName, "--help"}
	err := Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
