// Package inertia provides a minimal integration layer for GoNertia with Vite.
//
// It wires together:
//   - Vite dev server (HMR) or production manifest lookup.
//   - The root Inertia HTML template at resources/views/root.html.
//   - Optional SSR support via the Vite SSR server.
//
// # Quick start (development)
//
// This example shows a minimal setup with the default file layout used by this
// package. It starts the runtime services (Vite dev server + optional SSR),
// initializes the Inertia instance, and mounts the build handler.
//
//
//	pkg main
//
//	import (
//		"context"
//		"log"
//		"net/http"
//
//		"github.com/emilioforrer/go-stack/pkg/inertia"
//	)
//
//	func main() {
//		ctx := context.Background()
//
//		services, err := inertia.StartRuntimeServices(ctx, inertia.RuntimeConfig{
//			InertiaEnv: inertia.EnvDev,
//			SSREnabled: true,
//		})
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer services.Stop(ctx)
//
//	inertiaApp, err := inertia.InitInertia(inertia.InertiaConfig{})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	mux := http.NewServeMux()
//	mux.Handle("/build/", inertia.GetBuildHandler())
//
//	log.Fatal(http.ListenAndServe(":8000", mux))
//	}
//
// # Production (with SSR)
//
// In production, build assets and run the SSR server. The Inertia instance will
// read public/build/manifest.json and render using the root view template.
//
//
//	pkg main
//
//	import (
//		"context"
//		"log"
//		"net/http"
//
//		"github.com/emilioforrer/go-stack/pkg/inertia"
//	)
//
//	func main() {
//		ctx := context.Background()
//
//		services, err := inertia.StartRuntimeServices(ctx, inertia.RuntimeConfig{
//			InertiaEnv: inertia.EnvProd,
//			SSREnabled: true,
//		})
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer services.Stop(ctx)
//
//		inertiaApp, err := inertia.InitInertia(inertia.InertiaConfig{
//			InertiaEnv: inertia.EnvProd,
//		})
//
//		mux := http.NewServeMux()
//		mux.Handle("/build/", inertia.GetBuildHandler())
//		log.Fatal(http.ListenAndServe(":8000", mux))
//	}
//
// # Custom paths or embedded filesystems
//
// You can override the public/resources roots or provide fs.FS values when
// bundling files with embed.FS:
//
//	//go:embed public
//	var publicFS embed.FS
//
//	//go:embed resources
//	var resourcesFS embed.FS
//
//	func app() (*inertia.Inertia, error) {
//		return inertia.InitInertia(inertia.InertiaConfig{
//			InertiaEnv:    inertia.EnvProd,
//			PublicPath:    "public",
//			ResourcesPath: "resources",
//			PublicFS:      publicFS,
//			ResourcesFS:   resourcesFS,
//		})
//	}
package inertia
