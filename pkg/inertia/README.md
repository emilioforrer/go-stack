# pkg/inertia

Boot-ready Inertia.js integration for [go-stack](https://github.com/emilioforrer/go-stack) projects. Provides a `boot.Provider` that wires Vite, embedded assets, SSR, and the GoNertia adapter into your application lifecycle.

---

## Overview

This package embeds the `public/` and `resources/` directories with `embed.FS` so the Go binary is entirely self-contained in production. It also implements `boot.Provider` to handle initialization, route registration, and process lifecycle (start/stop Vite dev server and optional SSR Node process).

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    go-stack Bootstrapper                     │
│  ┌─────────────────┐  ┌───────────────────────────────────┐  │
│  │ ServerProvider  │  │        InertiaProvider          │  │
│  │                 │  │                                   │  │
│  │  Register()     │  │  Register()                       │  │
│  │   → router      │  │   → init Inertia instance         │  │
│  │   → api         │  │   → /build/ handler               │  │
│  │  Boot()         │  │   → scaffold files if missing     │  │
│  │   → start HTTP  │  │  Boot()                           │  │
│  │  Shutdown()     │  │   → start Vite / SSR processes    │  │
│  │   → stop HTTP   │  │  Shutdown()                       │  │
│  └─────────────────┘  │   → stop Vite / SSR processes     │  │
│                       └───────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Embedded Assets

The `assets.go` file embeds the `public/` and `resources/` directories into the binary. You can provide your own `embed.FS` or let the provider copy scaffold files to disk in development.

```go
//go:embed all:public
var PublicFS embed.FS

//go:embed all:resources
var ResourceFS embed.FS
```

---

## Usage

### 1. In your `cmd/app/serve.go`

```go
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
    "github.com/emilioforrer/go-stack/pkg/inertia"
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
    serveCmd.Flags().Bool("debug", false, "enable debug mode")

    // Inertia integration flags
    serveCmd.Flags().Bool("inertia-enabled", true, "enable Inertia.js integration")
    serveCmd.Flags().String("inertia-env", "dev", "Inertia environment: dev or prod")
    serveCmd.Flags().Bool("inertia-ssr-enabled", false, "enable SSR server")
    serveCmd.Flags().String("inertia-public-path", "public", "path to public directory")
    serveCmd.Flags().String("inertia-resources-path", "resources", "path to resources directory")

    _ = viper.BindPFlag("host", serveCmd.Flags().Lookup("host"))
    _ = viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
    _ = viper.BindPFlag("debug", serveCmd.Flags().Lookup("debug"))
    _ = viper.BindPFlag("inertia-enabled", serveCmd.Flags().Lookup("inertia-enabled"))
    _ = viper.BindPFlag("inertia-env", serveCmd.Flags().Lookup("inertia-env"))
    _ = viper.BindPFlag("inertia-ssr-enabled", serveCmd.Flags().Lookup("inertia-ssr-enabled"))
    _ = viper.BindPFlag("inertia-public-path", serveCmd.Flags().Lookup("inertia-public-path"))
    _ = viper.BindPFlag("inertia-resources-path", serveCmd.Flags().Lookup("inertia-resources-path"))
}

func runServe(ctx context.Context, _ *cobra.Command) error {
    i := do.New()

    serverOpts := provider.ServerOptions{
        Host:  viper.GetString("host"),
        Port:  viper.GetInt("port"),
        Debug: viper.GetBool("debug"),
    }

    inertiaOpts := inertia.ProviderOptions{
        Enabled:       viper.GetBool("inertia-enabled"),
        Env:           viper.GetString("inertia-env"),
        SSREnabled:    viper.GetBool("inertia-ssr-enabled"),
        PublicPath:    viper.GetString("inertia-public-path"),
        ResourcesPath: viper.GetString("inertia-resources-path"),
    }

    bootstrapper := newBootstrapper(
        i,
        provider.NewServerProvider(serverOpts),
        inertia.NewInertiaProvider(inertiaOpts), // Registered AFTER ServerProvider
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
```

### 2. Without boot (manual usage)

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/emilioforrer/go-stack/pkg/inertia"
)

func main() {
    ctx := context.Background()

    // Start Vite dev server (optional, for development)
    services, err := inertia.StartRuntimeServices(ctx, inertia.RuntimeConfig{
        InertiaEnv: inertia.EnvDev,
        SSREnabled: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer services.Stop(ctx)

    // Initialize Inertia (uses embedded FS in prod)
    app, err := inertia.InitInertia(inertia.InertiaConfig{
        InertiaEnv: inertia.EnvDev,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Serve build assets
    http.Handle("/build/", inertia.GetBuildHandler())

    log.Fatal(http.ListenAndServe(":8000", nil))
}
```

---

## Configuration

All configuration is passed via `inertia.ProviderOptions` when creating the provider.

| Option        | Default      | Description                                           |
|---------------|--------------|-------------------------------------------------------|
| `Enabled`     | `true`       | Enable/disable the Inertia integration                  |
| `Env`         | `"dev"`      | Environment mode (`"dev"` or `"prod"`)                |
| `SSREnabled`  | `false`      | Start the Node SSR server                             |
| `PublicPath`  | `"public"`   | Path to the public directory (contains `build/`, `hot`) |
| `ResourcesPath`| `"resources"` | Path to the resources directory (contains `views/root.html`) |

In production, set `Env` to `"prod"` and ensure you have built your frontend assets. The provider will automatically use the embedded `embed.FS`.

---

## Development Workflow

```bash
# Install Node dependencies
cd pkg/inertia && npm install

# Start Vite dev server (Terminal 1)
npm run dev:client

# Start the Go server (Terminal 2)
go run ./cmd/app serve
```

## Production Workflow

```bash
# Build frontend assets first
cd pkg/inertia && npm run build

# Build the Go binary (embeds assets at compile time)
cd ../..
go build -o bin/app ./cmd/app

# Run with production settings
./bin/app serve --inertia-env=prod
```

---

## API Reference

See [doc.go](doc.go) and the Go tests for comprehensive usage examples.

---

## License

MIT
