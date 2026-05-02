package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	appName  = "app"
	strDebug = "debug"
)

var (
	cfgFile     string
	userHomeDir = os.UserHomeDir
)

var rootCmd = &cobra.Command{
	Use:   appName,
	Short: "Start the go-stack template application",
	Long:  "A go-stack template application powered by Cobra and Viper.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func initCommands(rootCmd *cobra.Command) {
	if rootCmd.PersistentFlags().Lookup("config") == nil {
		rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.app.yaml)")
		rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
		_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	}

	var hasVersion bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "version" {
			hasVersion = true
		}
	}
	if !hasVersion {
		rootCmd.AddCommand(newVersionCmd())
		rootCmd.AddCommand(newCompletionCmd())
		rootCmd.AddCommand(serveCmd)
	}
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := userHomeDir()
		if err != nil {
			return fmt.Errorf("finding home directory: %w", err)
		}
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName("." + appName)
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix(strings.ToUpper(appName))
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if !errors.As(err, &notFoundErr) {
			return fmt.Errorf("reading config: %w", err)
		}
	}

	level := slog.LevelInfo
	switch strings.ToLower(viper.GetString("log-level")) {
	case strDebug:
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	return nil
}
