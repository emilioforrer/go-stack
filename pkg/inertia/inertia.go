package inertia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gonertia "github.com/romsar/gonertia/v2"
)

var (
	// ErrAssetNotFound is returned when a requested asset is not found in the manifest.
	ErrAssetNotFound = errors.New("asset not found")

	// ssrAddr holds the SSR server address. Modified via SetSSRAddr for testing.
	ssrAddr = "http://localhost:13714"

	// runCommandFactory is the function used to create and start runners. Swapped in tests.
	// Use getRunCommandFactory/setRunCommandFactory for thread-safe access.
	runCommandFactory runnerFactoryFunc = defaultRunCommand
	runCommandMu      sync.RWMutex
)

type runnerFactoryFunc func(ctx context.Context, name string, args ...string) (Runner, error)

const (
	// EnvDev is the development environment identifier.
	EnvDev = "dev"
	// EnvProd is the production environment identifier.
	EnvProd = "prod"

	defaultPublicPath    = "public"
	defaultResourcesPath = "resources"
	buildURLPrefix       = "/build/"
)

// InertiaConfig controls how the Inertia instance is initialized.
type InertiaConfig struct {
	InertiaEnv    string // "dev" or "prod"
	PublicPath    string // defaults to "public"
	ResourcesPath string // defaults to "resources"
	PublicFS      fs.FS // optional embedded filesystem for production
	ResourcesFS   fs.FS // optional embedded filesystem for production
	SSREnabled    bool   // whether to enable server-side rendering
}

// RuntimeConfig controls which background Node processes are started.
type RuntimeConfig struct {
	InertiaEnv string
	SSREnabled bool
}

// Runner manages the lifecycle of a background process.
type Runner interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
}

// RuntimeServices holds running background processes (Vite, SSR).
type RuntimeServices struct {
	runners []Runner
}

// Stop stops all background Node processes.
func (r *RuntimeServices) Stop(ctx context.Context) {
	for _, runner := range r.runners {
		if err := runner.Stop(ctx); err != nil {
			slog.Error("failed to stop runner", "name", runner.Name(), "error", err)
		}
	}
}

// InitInertia creates a Gonertia instance based on the provided configuration.
// It auto-detects dev vs prod mode based on InertiaEnv or presence of public/hot.
func InitInertia(config InertiaConfig) (*gonertia.Inertia, error) {
	publicPath := normalizePublicPath(config.PublicPath)
	resourcesPath := normalizeResourcesPath(config.ResourcesPath)
	publicFS := resolvePublicFS(config)
	resourcesFS := resolveResourcesFS(config)
	viteHotFile := path.Join(publicPath, "hot")
	rootViewFile := path.Join(resourcesPath, "views", "root.html")
	publicIsOS := config.PublicFS == nil

	env := normalizeInertiaEnv(config.InertiaEnv)
	if env == EnvDev {
		return initDevInertia(rootViewFile, viteHotFile, publicFS, resourcesFS, config.SSREnabled)
	}
	if hasViteHotFile(viteHotFile, publicFS) {
		return initDevInertia(rootViewFile, viteHotFile, publicFS, resourcesFS, config.SSREnabled)
	}

	return initProdInertia(rootViewFile, publicPath, publicFS, resourcesFS, publicIsOS, config.SSREnabled)
}

// GetBuildHandler returns a file server for /build/ that serves files from public/build/.
func GetBuildHandler() http.Handler {
	return GetBuildHandlerWithConfig(InertiaConfig{})
}

// GetBuildHandlerWithConfig returns a file server for /build/ that serves files from
// the configured public/build directory or embedded filesystem.
func GetBuildHandlerWithConfig(config InertiaConfig) http.Handler {
	publicPath := normalizePublicPath(config.PublicPath)
	publicFS := resolvePublicFS(config)
	buildFS, err := fs.Sub(publicFS, path.Join(publicPath, "build"))
	if err != nil {
		slog.Error("failed to create build filesystem", "error", err)

		return http.NotFoundHandler()
	}

	return http.StripPrefix(buildURLPrefix, http.FileServer(http.FS(buildFS)))
}

// StartRuntimeServices starts background Node processes (Vite dev server, optional SSR).
func StartRuntimeServices(ctx context.Context, config RuntimeConfig) (*RuntimeServices, error) {
	env := normalizeInertiaEnv(config.InertiaEnv)
	switch env {
	case EnvDev:
		return startDevServices(ctx, config.SSREnabled)
	case EnvProd:
		return startProdServices(ctx, config.SSREnabled)
	default:
		return nil, fmt.Errorf("unsupported inertia env: %s", env)
	}
}

func initDevInertia(rootViewFile, viteHotFile string, publicFS, resourcesFS fs.FS, ssrEnabled bool) (*gonertia.Inertia, error) {
	i, err := newInertia(rootViewFile, resourcesFS, ssrEnabled)
	if err != nil {
		return nil, err
	}
	i.ShareTemplateFunc("vite", viteHotTemplateFunc(viteHotFile, publicFS))
	i.ShareTemplateData("hmr", true)

	return i, nil
}

func initProdInertia(rootViewFile, publicPath string, publicFS, resourcesFS fs.FS, publicIsOS bool, ssrEnabled bool) (*gonertia.Inertia, error) {
	manifestPath := path.Join(publicPath, "build", "manifest.json")
	if err := ensureViteManifest(publicFS, publicPath, manifestPath, publicIsOS); err != nil {
		slog.Error("failed to ensure vite manifest", "error", err)
	}

	i, err := newInertia(rootViewFile, resourcesFS, ssrEnabled)
	if err != nil {
		return nil, err
	}
	viteFunc := viteProdFunc(publicFS, manifestPath, buildURLPrefix)
	if viteFunc == nil {
		slog.Error("vite manifest could not be loaded")
	}

	i.ShareTemplateFunc("vite", viteFunc)

	return i, nil
}

func ensureViteManifest(publicFS fs.FS, publicPath, manifestPath string, publicIsOS bool) error {
	_, statErr := fs.Stat(publicFS, manifestPath)
	if statErr == nil {
		return nil
	}

	if !publicIsOS {
		return fmt.Errorf("vite manifest not found in embedded public filesystem at %s: %w", manifestPath, statErr)
	}

	sourcePath := path.Join(publicPath, "build", ".vite", "manifest.json")
	if renameErr := os.Rename(sourcePath, manifestPath); renameErr != nil {
		return fmt.Errorf("failed to move vite manifest from %s to %s: %w", sourcePath, manifestPath, renameErr)
	}

	return nil
}

func newInertia(rootViewFile string, resourcesFS fs.FS, ssrEnabled bool) (*gonertia.Inertia, error) {
	rootViewPath := filepath.Clean(rootViewFile)
	if _, statErr := os.Stat(rootViewPath); statErr != nil {
		embeddedData, readErr := fs.ReadFile(resourcesFS, rootViewFile)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read root view: %w", readErr)
		}

		tempFile, tempErr := os.CreateTemp("", "inertia-root-*.html")
		if tempErr != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", tempErr)
		}
		defer tempFile.Close() //nolint:errcheck // Best-effort close.

		if _, writeErr := tempFile.Write(embeddedData); writeErr != nil {
			return nil, fmt.Errorf("failed to write temp file: %w", writeErr)
		}

		rootViewPath = tempFile.Name()
	}

	opts := []gonertia.Option{}
	if ssrEnabled {
		opts = append(opts, gonertia.WithSSR(ssrAddr))
	}

	i, initErr := gonertia.NewFromFile(
		rootViewPath,
		opts...,
	)
	if initErr != nil {
		return nil, fmt.Errorf("failed to create inertia instance: %w", initErr)
	}

	return i, nil
}

// SetSSRAddr sets the SSR server address. Used mainly for testing.
func SetSSRAddr(addr string) {
	ssrAddr = addr
}

func viteHotTemplateFunc(viteHotFile string, publicFS fs.FS) func(entry string) (string, error) {
	return func(entry string) (string, error) {
		content, readErr := fs.ReadFile(publicFS, viteHotFile)
		if readErr != nil {
			return "", fmt.Errorf("failed to read hot file: %w", readErr)
		}

		url := strings.TrimSpace(string(content))
		url = normalizeHotURL(url)

		if entry != "" && !strings.HasPrefix(entry, "/") {
			entry = "/" + entry
		}

		return url + entry, nil
	}
}

func normalizeHotURL(url string) string {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		idx := strings.Index(url, ":")
		if idx >= 0 {
			url = url[idx+1:]
		}
	} else {
		url = "//localhost:8000"
	}

	return url
}

func viteProdFunc(publicFS fs.FS, manifestPath, buildDir string) func(p string) (string, error) {
	manifestBytes, err := fs.ReadFile(publicFS, manifestPath)
	if err != nil {
		slog.Error("cannot open vite manifest file", "error", err)

		return nil
	}

	viteAssets, err := parseManifest(manifestBytes)
	if err != nil {
		slog.Error("cannot parse vite manifest file", "error", err)

		return nil
	}

	return func(p string) (string, error) {
		if val, ok := viteAssets[p]; ok {
			return path.Join("/", buildDir, val.File), nil
		}

		return "", fmt.Errorf("%w: %q", ErrAssetNotFound, p)
	}
}

type manifestEntry struct {
	File   string `json:"file"`
	Source string `json:"src"`
}

func parseManifest(data []byte) (map[string]*manifestEntry, error) {
	viteAssets := make(map[string]*manifestEntry)
	if err := json.Unmarshal(data, &viteAssets); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}

	return viteAssets, nil
}

func startDevServices(ctx context.Context, ssrEnabled bool) (*RuntimeServices, error) {
	services := &RuntimeServices{}

	client, err := runClientDevServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("start client dev server: %w", err)
	}
	services.runners = append(services.runners, client)

	if err := waitForFile(ctx, "./public/hot", 10*time.Second); err != nil {
		services.Stop(ctx)

		return nil, fmt.Errorf("wait for hot file: %w", err)
	}

	if !ssrEnabled {
		return services, nil
	}

	ssrBuild, err := runSSRBuildWatch(ctx)
	if err != nil {
		services.Stop(ctx)

		return nil, fmt.Errorf("start ssr build watch: %w", err)
	}
	services.runners = append(services.runners, ssrBuild)

	if err := waitForFile(ctx, "./bootstrap/assets/ssr.js", 10*time.Second); err != nil {
		services.Stop(ctx)

		return nil, fmt.Errorf("wait for ssr bundle: %w", err)
	}

	ssr, err := runSSR(ctx)
	if err != nil {
		services.Stop(ctx)

		return nil, fmt.Errorf("start ssr server: %w", err)
	}
	services.runners = append(services.runners, ssr)

	return services, nil
}

func startProdServices(ctx context.Context, ssrEnabled bool) (*RuntimeServices, error) {
	services := &RuntimeServices{}
	if !ssrEnabled {
		return services, nil
	}

	if err := ensureSSRBuild(); err != nil {
		return nil, err
	}

	ssr, err := runSSR(ctx)
	if err != nil {
		return nil, fmt.Errorf("start ssr server: %w", err)
	}
	services.runners = append(services.runners, ssr)

	return services, nil
}

func ensureSSRBuild() error {
	if _, err := os.Stat("./bootstrap/assets/ssr.js"); err != nil {
		return fmt.Errorf("missing SSR bundle at ./bootstrap/assets/ssr.js: run `npm run build` first: %w", err)
	}

	return nil
}

func waitForFile(ctx context.Context, filePath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(filePath); err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for file: %s", filePath)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func runClientDevServer(ctx context.Context) (Runner, error) {
	return runCommand(ctx, "npm", "run", "dev:client")
}

func runSSRBuildWatch(ctx context.Context) (Runner, error) {
	return runCommand(ctx, "npm", "run", "dev:ssr:build")
}

func runSSR(ctx context.Context) (Runner, error) {
	return runCommand(ctx, "npm", "run", "ssr")
}

func defaultRunCommand(ctx context.Context, name string, args ...string) (Runner, error) {
	projectPath, err := getProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("get project root: %w", err)
	}

	runner := newOSProcessRunner(ProcessConfig{
		Name: name,
		Args: args,
		Dir:  projectPath,
	})

	if startErr := runner.Start(ctx); startErr != nil {
		return nil, fmt.Errorf("start %s %s: %w", name, strings.Join(args, " "), startErr)
	}

	return runner, nil
}

func getRunCommandFactory() runnerFactoryFunc {
	runCommandMu.RLock()
	defer runCommandMu.RUnlock()
	return runCommandFactory
}

func setRunCommandFactory(f runnerFactoryFunc) {
	runCommandMu.Lock()
	defer runCommandMu.Unlock()
	runCommandFactory = f
}

func runCommand(ctx context.Context, name string, args ...string) (Runner, error) {
	return getRunCommandFactory()(ctx, name, args...)
}

func getProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	return cwd, nil
}

func normalizeInertiaEnv(value string) string {
	env := strings.ToLower(strings.TrimSpace(value))
	if env == "" {
		return EnvDev
	}

	return env
}

func normalizePublicPath(value string) string {
	pathValue := strings.TrimSpace(value)
	if pathValue == "" {
		return defaultPublicPath
	}

	return strings.Trim(pathValue, "/")
}

func normalizeResourcesPath(value string) string {
	pathValue := strings.TrimSpace(value)
	if pathValue == "" {
		return defaultResourcesPath
	}

	return strings.Trim(pathValue, "/")
}

func resolvePublicFS(config InertiaConfig) fs.FS {
	if config.PublicFS != nil {
		return config.PublicFS
	}

	return os.DirFS(".")
}

func resolveResourcesFS(config InertiaConfig) fs.FS {
	if config.ResourcesFS != nil {
		return config.ResourcesFS
	}

	return os.DirFS(".")
}

func hasViteHotFile(viteHotFile string, publicFS fs.FS) bool {
	_, err := fs.Stat(publicFS, viteHotFile)

	return err == nil
}

// ProcessConfig holds the configuration for a background process.
type ProcessConfig struct {
	Name string
	Args []string
	Dir  string
}

// OSProcessRunner implements Runner using os/exec for real process management.
type OSProcessRunner struct {
	config ProcessConfig
	cmd    *exec.Cmd
	mu     sync.Mutex
}

func newOSProcessRunner(config ProcessConfig) *OSProcessRunner {
	return &OSProcessRunner{
		config: config,
	}
}

func (r *OSProcessRunner) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil {
		return errors.New("runner already started")
	}

	cmd := exec.CommandContext(ctx, r.config.Name, r.config.Args...)
	if r.config.Dir != "" {
		cmd.Dir = r.config.Dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if startErr := cmd.Start(); startErr != nil {
		return fmt.Errorf("start command: %w", startErr)
	}

	r.cmd = cmd

	go func() {
		if waitErr := cmd.Wait(); waitErr != nil {
			slog.Error("command exited with error", "name", r.config.Name, "error", waitErr)
		}
	}()

	return nil
}

func (r *OSProcessRunner) Stop(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	return r.cmd.Process.Kill()
}

func (r *OSProcessRunner) Name() string {
	return r.config.Name
}
