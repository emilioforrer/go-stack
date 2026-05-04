//go:build ignore

// This is a standalone example of how to use the inertia package.
//
// For a full boot-provider example, see the README.md.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/emilioforrer/go-stack/pkg/inertia"
	gonertia "github.com/romsar/gonertia/v2"
)

func main() {
	ctx := context.Background()
	ssrEnanbled := true
	envDev := inertia.EnvDev

	// if envDev ==  prod, need to run npm run build  # generates public/build/ with the manifest

	services, err := inertia.StartRuntimeServices(ctx, inertia.RuntimeConfig{
		InertiaEnv: envDev,
		SSREnabled: ssrEnanbled,
	})
	if err != nil {
		slog.Error("failed to start runtime services", "error", err)
		os.Exit(1)
	}
	defer services.Stop(ctx)

	app, err := inertia.InitInertia(inertia.InertiaConfig{
		InertiaEnv: envDev,
		SSREnabled: ssrEnanbled,
	})
	if err != nil {
		slog.Error("failed to initialize inertia", "error", err)
		os.Exit(1)
	}

	http.Handle("/build/", inertia.GetBuildHandler())

	http.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if err := app.Render(w, r, "Home/Index", gonertia.Props{
			"text": "Hello from go-stack/pkg/inertia!",
		}); err != nil {
			slog.Error("inertia render error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	addr := ":8000"
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
