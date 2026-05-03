package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/emilioforrer/go-stack/internal/scaffold/template/boot/provider"
	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var newBootstrapper = func(i do.Injector, providers ...boot.Provider) boot.Bootstrapper {
	return boot.NewDefaultBootstrapper(i, providers...)
}

var serveCmd = func() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return runServe(ctx, cmd)
		},
	}
	serveCmdFlags(cmd)
	return cmd
}()

func serveCmdFlags(serveCmd *cobra.Command) {
	serveCmd.Flags().String("host", "0.0.0.0", "hostname to bind to")
	serveCmd.Flags().IntP("port", "p", 8888, "port to listen on")
	serveCmd.Flags().Bool(strDebug, false, "enable debug mode")
	_ = viper.BindPFlag("host", serveCmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag(strDebug, serveCmd.Flags().Lookup(strDebug))
}

func runServe(ctx context.Context, _ *cobra.Command) error {
	i := do.New()

	opts := provider.ServerOptions{
		Host:  viper.GetString("host"),
		Port:  viper.GetInt("port"),
		Debug: viper.GetBool(strDebug),
	}

	bootstrapper := newBootstrapper(
		i,
		provider.NewServerProvider(opts),
	)

	do.ProvideValue(i, bootstrapper)

	if err := bootstrapper.Register(ctx); err != nil {
		return fmt.Errorf("failed to register dependencies: %w", err)
	}

	if err := bootstrapper.Boot(ctx); err != nil {
		return fmt.Errorf("failed to boot application: %w", err)
	}

	<-ctx.Done()
	slog.Info("shutdown signal received")

	if err := bootstrapper.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown application: %w", err)
	}

	return nil
}
