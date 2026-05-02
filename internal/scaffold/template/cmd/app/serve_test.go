package main

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type mockBootstrapper struct {
	registerErr error
	bootErr     error
	shutdownErr error
}

func (m *mockBootstrapper) Container() boot.Container {
	return nil
}

func (m *mockBootstrapper) Providers() []boot.Provider {
	return nil
}

func (m *mockBootstrapper) Register(ctx context.Context) error {
	return m.registerErr
}

func (m *mockBootstrapper) Boot(ctx context.Context) error {
	return m.bootErr
}

func (m *mockBootstrapper) Shutdown(ctx context.Context) error {
	return m.shutdownErr
}

func TestServeCmdFlags(t *testing.T) {
	cmd := &cobra.Command{}
	serveCmdFlags(cmd)

	host, err := cmd.Flags().GetString("host")
	if err != nil || host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", host)
	}

	port, err := cmd.Flags().GetInt("port")
	if err != nil || port != 8888 {
		t.Errorf("expected port 8888, got %d", port)
	}

	debug, err := cmd.Flags().GetBool(strDebug)
	if err != nil || debug != false {
		t.Errorf("expected debug false, got %v", debug)
	}
}

func TestRunServe(t *testing.T) {
	viper.Set("host", "127.0.0.1")
	viper.Set("port", 0) // dynamic port to avoid conflicts
	viper.Set(strDebug, true)

	// Create a context that will cancel itself after a short delay
	// to allow the server to boot, then unblock <-ctx.Done(), and shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := runServe(ctx, &cobra.Command{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServeCmdRunE(t *testing.T) {
	viper.Set("host", "127.0.0.1")
	viper.Set("port", 0)
	viper.Set(strDebug, true)

	cmd := serveCmd
	cmd.SetArgs([]string{})

	go func() {
		time.Sleep(50 * time.Millisecond)
		proc, _ := os.FindProcess(os.Getpid())
		_ = proc.Signal(syscall.SIGTERM)
	}()

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunServeErrors(t *testing.T) {
	oldNewBootstrapper := newBootstrapper
	defer func() { newBootstrapper = oldNewBootstrapper }()

	tests := []struct {
		name        string
		registerErr error
		bootErr     error
		shutdownErr error
		expectedErr string
	}{
		{
			name:        "register error",
			registerErr: errors.New("register error"),
			expectedErr: "failed to register dependencies: register error",
		},
		{
			name:        "boot error",
			bootErr:     errors.New("boot error"),
			expectedErr: "failed to boot application: boot error",
		},
		{
			name:        "shutdown error",
			shutdownErr: errors.New("shutdown error"),
			expectedErr: "failed to shutdown application: shutdown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newBootstrapper = func(i do.Injector, providers ...boot.Provider) boot.Bootstrapper {
				return &mockBootstrapper{
					registerErr: tt.registerErr,
					bootErr:     tt.bootErr,
					shutdownErr: tt.shutdownErr,
				}
			}

			// Use an already canceled context so <-ctx.Done() doesn't block
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := runServe(ctx, &cobra.Command{})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err.Error() != tt.expectedErr {
				t.Errorf("expected %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}
