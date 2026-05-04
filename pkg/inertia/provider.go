package inertia

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/emilioforrer/go-stack/pkg/httpsvr"
	gonertia "github.com/romsar/gonertia/v2"
	"github.com/samber/do/v2"
)

// ProviderOptions holds the configuration for the InertiaProvider.
type ProviderOptions struct {
	Enabled       bool
	Env           string
	SSREnabled    bool
	PublicPath    string
	ResourcesPath string
}

// InertiaProvider implements boot.Provider to integrate Inertia.js into the application lifecycle.
type InertiaProvider struct {
	boot.DefaultProvider
	options  ProviderOptions
	instance *gonertia.Inertia
	services *RuntimeServices
}

var _ boot.Provider = (*InertiaProvider)(nil)

// NewInertiaProvider creates a new InertiaProvider with the given options.
func NewInertiaProvider(opts ProviderOptions) *InertiaProvider {
	return &InertiaProvider{options: opts}
}

// Register initializes the Inertia instance, copies scaffold files if needed, and registers routes.
func (p *InertiaProvider) Register(_ context.Context, container boot.Container) error {
	if !p.options.Enabled {
		slog.Info("inertia provider disabled")
		return nil
	}

	if err := p.ensureScaffoldFiles(); err != nil {
		return fmt.Errorf("inertia provider: ensure scaffold files: %w", err)
	}

    inertiaConfig := InertiaConfig{
        InertiaEnv:    p.options.Env,
        PublicPath:    p.options.PublicPath,
        ResourcesPath: p.options.ResourcesPath,
        SSREnabled:    p.options.SSREnabled,
    }

    var err error
    p.instance, err = InitInertia(inertiaConfig)
    if err != nil {
        return fmt.Errorf("inertia provider: failed to initialize inertia instance: %w", err)
    }

	if p.options.Env == EnvProd {
		inertiaConfig.PublicFS = PublicFS
		inertiaConfig.ResourcesFS = ResourceFS
	}

	p.instance, err = InitInertia(inertiaConfig)
	if err != nil {
		return fmt.Errorf("inertia provider: failed to initialize inertia instance: %w", err)
	}

	if err := p.registerRoutes(container, inertiaConfig); err != nil {
		return err
	}

	do.ProvideValue(container.Injector(), p.instance)

	return nil
}

// Boot starts background runtime services (Vite dev server, SSR) if configured.
func (p *InertiaProvider) Boot(ctx context.Context, _ boot.Container) error {
	if !p.options.Enabled {
		return nil
	}

	services, err := StartRuntimeServices(ctx, RuntimeConfig{
		InertiaEnv: p.options.Env,
		SSREnabled: p.options.SSREnabled,
	})
	if err != nil {
		return fmt.Errorf("inertia provider: failed to start runtime services: %w", err)
	}

	p.services = services
	slog.Info("inertia runtime services started", "env", p.options.Env, "ssr", p.options.SSREnabled)

	return nil
}

// Shutdown stops background runtime services.
func (p *InertiaProvider) Shutdown(_ context.Context, _ boot.Container) error {
	if p.services == nil {
		return nil
	}

	slog.Info("stopping inertia runtime services...")
	p.services.Stop(context.Background())

	return nil
}

// Instance returns the initialized Gonertia instance (may be nil if disabled).
func (p *InertiaProvider) Instance() *gonertia.Inertia {
	return p.instance
}

func (p *InertiaProvider) registerRoutes(container boot.Container, config InertiaConfig) error {
	router, err := do.Invoke[*httpsvr.DefaultRouter](container.Injector())
	if err != nil {
		slog.Warn("inertia provider: router not found in container, skipping route registration", "error", err)
		return nil
	}

	router.HandleFunc("/build/", GetBuildHandlerWithConfig(config).ServeHTTP)

	// Register example routes when scaffold files are present
	homeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if renderErr := p.instance.Render(w, r, "Home/Index", gonertia.Props{
			"text": "Inertia.js with React and Go!",
		}); renderErr != nil {
			slog.Error("inertia render error", "error", renderErr)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if renderErr := p.instance.Render(w, r, "Admin/Dashboard", gonertia.Props{
			"text": "Admin Dashboard",
		}); renderErr != nil {
			slog.Error("inertia render error", "error", renderErr)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	router.HandleFunc("GET /{$}", p.instance.Middleware(homeHandler).ServeHTTP)
	router.HandleFunc("GET /admin/dashboard", p.instance.Middleware(adminHandler).ServeHTTP)

	return nil
}

func (p *InertiaProvider) ensureScaffoldFiles() error {
	publicEmbedFS := getEmbeddedPublicFS()()
	if _, statErr := os.Stat(p.options.PublicPath); os.IsNotExist(statErr) {
		slog.Info("inertia provider: public directory not found, creating from embedded assets")
		if err := copyEmbeddedDir(publicEmbedFS, "public", p.options.PublicPath); err != nil {
			return fmt.Errorf("copy public assets: %w", err)
		}
	}

	resourceEmbedFS := getEmbeddedResourcesFS()()
	if _, statErr := os.Stat(p.options.ResourcesPath); os.IsNotExist(statErr) {
		slog.Info("inertia provider: resources directory not found, creating from embedded assets")
		if err := copyEmbeddedDir(resourceEmbedFS, "resources", p.options.ResourcesPath); err != nil {
			return fmt.Errorf("copy resources: %w", err)
		}
	}

	return nil
}

var (
	// embeddedPublicFS returns the embedded public filesystem.
	// Overridden in tests via setEmbeddedPublicFS.
	embeddedPublicFS   = func() embedFS { return PublicFS }
	embeddedPublicFSMu sync.Mutex

	// embeddedResourcesFS returns the embedded resources filesystem.
	// Overridden in tests via setEmbeddedResourcesFS.
	embeddedResourcesFS   = func() embedFS { return ResourceFS }
	embeddedResourcesFSMu sync.Mutex
)

func getEmbeddedPublicFS() func() embedFS {
	embeddedPublicFSMu.Lock()
	defer embeddedPublicFSMu.Unlock()
	return embeddedPublicFS
}

func setEmbeddedPublicFS(f func() embedFS) {
	embeddedPublicFSMu.Lock()
	defer embeddedPublicFSMu.Unlock()
	embeddedPublicFS = f
}

func getEmbeddedResourcesFS() func() embedFS {
	embeddedResourcesFSMu.Lock()
	defer embeddedResourcesFSMu.Unlock()
	return embeddedResourcesFS
}

func setEmbeddedResourcesFS(f func() embedFS) {
	embeddedResourcesFSMu.Lock()
	defer embeddedResourcesFSMu.Unlock()
	embeddedResourcesFS = f
}

// embedFS is the interface needed for embedded file operations.
type embedFS interface {
	ReadDir(name string) ([]os.DirEntry, error)
	ReadFile(name string) ([]byte, error)
}

func copyEmbeddedDir(fs embedFS, src, dst string) error {
	entries, err := fs.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read embedded dir %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if mkdirErr := os.MkdirAll(dstPath, 0o755); mkdirErr != nil {
				return fmt.Errorf("mkdir %s: %w", dstPath, mkdirErr)
			}

			if copyErr := copyEmbeddedDirRecursive(fs, srcPath, dstPath); copyErr != nil {
				return copyErr
			}

			continue
		}

		data, readErr := fs.ReadFile(srcPath)
		if readErr != nil {
			return fmt.Errorf("read embedded file %s: %w", srcPath, readErr)
		}

		if writeErr := os.WriteFile(dstPath, data, 0o644); writeErr != nil {
			return fmt.Errorf("write file %s: %w", dstPath, writeErr)
		}
	}

	return nil
}

func copyEmbeddedDirRecursive(fs embedFS, src, dst string) error {
	entries, err := fs.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read embedded dir %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if mkdirErr := os.MkdirAll(dstPath, 0o755); mkdirErr != nil {
				return fmt.Errorf("mkdir %s: %w", dstPath, mkdirErr)
			}

			if copyErr := copyEmbeddedDirRecursive(fs, srcPath, dstPath); copyErr != nil {
				return copyErr
			}

			continue
		}

		data, readErr := fs.ReadFile(srcPath)
		if readErr != nil {
			return fmt.Errorf("read embedded file %s: %w", srcPath, readErr)
		}

		if writeErr := os.WriteFile(dstPath, data, 0o644); writeErr != nil {
			return fmt.Errorf("write file %s: %w", dstPath, writeErr)
		}
	}

	return nil
}
