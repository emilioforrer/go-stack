package inertia

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/emilioforrer/go-stack/pkg/httpsvr"
	"github.com/samber/do/v2"
)

type mockContainer struct {
	injector do.Injector
}

func (m *mockContainer) Injector() do.Injector {
	return m.injector
}

func newMockContainer() boot.Container {
	return &mockContainer{
		injector: do.New(),
	}
}

func TestNewInertiaProvider(t *testing.T) {
	t.Parallel()
	opts := ProviderOptions{
		Enabled:       true,
		Env:           EnvDev,
		SSREnabled:    false,
		PublicPath:    "test-public",
		ResourcesPath: "test-resources",
	}

	provider := NewInertiaProvider(opts)
	requireNotNil(t, provider)
	requireEqual(t, opts, provider.options)
	requireNil(t, provider.instance)
	requireNil(t, provider.services)
}

func TestInertiaProvider_Register_Disabled(t *testing.T) {
	t.Parallel()
	provider := NewInertiaProvider(ProviderOptions{Enabled: false})
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Register(ctx, container)
	requireNoError(t, err)
	requireNil(t, provider.instance)
}

func TestInertiaProvider_Register_WithExistingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")
	resourcesDir := filepath.Join(dir, "resources")

	requireMkdirAll(t, filepath.Join(publicDir, "build"), 0o755)
	requireWriteFile(t, filepath.Join(publicDir, "build", "manifest.json"), []byte(`{}`), 0o644)
	requireMkdirAll(t, filepath.Join(resourcesDir, "views"), 0o755)
	requireWriteFile(t, filepath.Join(resourcesDir, "views", "root.html"), []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvDev,
		PublicPath:    "public",
		ResourcesPath: "resources",
	})

	router := httpsvr.NewDefaultRouter()
	container := newMockContainer()
	do.ProvideValue(container.Injector(), router)

	ctx := context.Background()
	err := provider.Register(ctx, container)
	requireNoError(t, err)
	requireNotNil(t, provider.instance)
}

func TestInertiaProvider_Register_WithoutRouter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")
	resourcesDir := filepath.Join(dir, "resources")

	requireMkdirAll(t, filepath.Join(publicDir, "build"), 0o755)
	requireWriteFile(t, filepath.Join(publicDir, "build", "manifest.json"), []byte(`{}`), 0o644)
	requireMkdirAll(t, filepath.Join(resourcesDir, "views"), 0o755)
	requireWriteFile(t, filepath.Join(resourcesDir, "views", "root.html"), []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvDev,
		PublicPath:    "public",
		ResourcesPath: "resources",
	})

	container := newMockContainer()
	ctx := context.Background()

	err := provider.Register(ctx, container)
	requireNoError(t, err)
	requireNotNil(t, provider.instance)
}

func TestInertiaProvider_Boot_Disabled(t *testing.T) {
	t.Parallel()
	provider := NewInertiaProvider(ProviderOptions{Enabled: false})
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Boot(ctx, container)
	requireNoError(t, err)
	requireNil(t, provider.services)
}

func TestInertiaProvider_Shutdown_NilServices(t *testing.T) {
	t.Parallel()
	provider := NewInertiaProvider(ProviderOptions{Enabled: true})
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Shutdown(ctx, container)
	requireNoError(t, err)
}

func TestInertiaProvider_Shutdown_WithServices(t *testing.T) {
	t.Parallel()
	provider := NewInertiaProvider(ProviderOptions{Enabled: true})
	provider.services = &RuntimeServices{}
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Shutdown(ctx, container)
	requireNoError(t, err)
}

func TestInertiaProvider_Instance(t *testing.T) {
	t.Parallel()
	provider := NewInertiaProvider(ProviderOptions{Enabled: true})
	requireNil(t, provider.Instance())
}

func TestInertiaProvider_RegisterRoutes_RouterNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")
	resourcesDir := filepath.Join(dir, "resources")

	requireMkdirAll(t, filepath.Join(publicDir, "build"), 0o755)
	requireWriteFile(t, filepath.Join(publicDir, "build", "manifest.json"), []byte(`{}`), 0o644)
	requireMkdirAll(t, filepath.Join(resourcesDir, "views"), 0o755)
	requireWriteFile(t, filepath.Join(resourcesDir, "views", "root.html"), []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvDev,
		PublicPath:    "public",
		ResourcesPath: "resources",
	})

	container := newMockContainer()
	ctx := context.Background()

	err := provider.Register(ctx, container)
	requireNoError(t, err)
}

func TestInertiaProvider_EnsureScaffoldFiles(t *testing.T) {
	t.Parallel()
	t.Run("paths already exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		publicDir := filepath.Join(dir, "public")
		resourcesDir := filepath.Join(dir, "resources")
		requireMkdirAll(t, publicDir, 0o755)
		requireMkdirAll(t, resourcesDir, 0o755)

		provider := NewInertiaProvider(ProviderOptions{
			Enabled:       true,
			Env:           EnvDev,
			PublicPath:    publicDir,
			ResourcesPath: resourcesDir,
		})

		err := provider.ensureScaffoldFiles()
		requireNoError(t, err)
	})
}

func TestInertiaProvider_Boot(t *testing.T) {
	t.Parallel()
	t.Run("boot with prod env and no ssr", func(t *testing.T) {
		t.Parallel()
		provider := NewInertiaProvider(ProviderOptions{
			Enabled:    true,
			Env:        EnvProd,
			SSREnabled: false,
		})
		container := newMockContainer()
		ctx := context.Background()

		err := provider.Boot(ctx, container)
		requireNoError(t, err)
		requireNotNil(t, provider.services)
	})
}

func TestCopyEmbeddedDir(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mockFS := &mockEmbedFS{
			files: map[string][]byte{
				"test/foo.txt": []byte("hello"),
			},
			dirs: map[string][]os.DirEntry{
				"test": {mockDir("foo.txt", false)},
			},
		}

		dst := t.TempDir()
		err := copyEmbeddedDir(mockFS, "test", dst)
		requireNoError(t, err)

		content, err := os.ReadFile(filepath.Join(dst, "foo.txt"))
		requireNoError(t, err)
		requireEqual(t, "hello", string(content))
	})

	t.Run("read dir error", func(t *testing.T) {
		t.Parallel()
		mockFS := &mockEmbedFS{readDirErr: errors.New("boom")}
		err := copyEmbeddedDir(mockFS, "test", t.TempDir())
		requireError(t, err)
		assertContains(t, err.Error(), "read embedded dir")
	})

	t.Run("read file error", func(t *testing.T) {
		t.Parallel()
		mockFS := &mockEmbedFS{
			dirs: map[string][]os.DirEntry{
				"test": {mockDir("foo.txt", false)},
			},
			readFileErr: errors.New("boom"),
		}

		err := copyEmbeddedDir(mockFS, "test", t.TempDir())
		requireError(t, err)
		assertContains(t, err.Error(), "read embedded file")
	})

	t.Run("mkdir error", func(t *testing.T) {
		t.Parallel()
		mockFS := &mockEmbedFS{
			dirs: map[string][]os.DirEntry{
				"test": {mockDir("subdir", true)},
			},
		}

		dst := t.TempDir()
		requireWriteFile(t, filepath.Join(dst, "subdir"), []byte("x"), 0o644)

		err := copyEmbeddedDir(mockFS, "test", dst)
		requireError(t, err)
	})
}

func TestCopyEmbeddedDirRecursive(t *testing.T) {
	t.Parallel()

	t.Run("read dir error", func(t *testing.T) {
		t.Parallel()
		mockFS := &mockEmbedFS{readDirErr: errors.New("boom")}
		err := copyEmbeddedDirRecursive(mockFS, "test", t.TempDir())
		requireError(t, err)
	})

	t.Run("mkdir error", func(t *testing.T) {
		t.Parallel()
		mockFS := &mockEmbedFS{
			dirs: map[string][]os.DirEntry{
				"test": {mockDir("subdir", true)},
			},
		}

		dst := t.TempDir()
		requireWriteFile(t, filepath.Join(dst, "subdir"), []byte("x"), 0o644)

		err := copyEmbeddedDirRecursive(mockFS, "test", dst)
		requireError(t, err)
	})

	t.Run("write file error", func(t *testing.T) {
		t.Parallel()
		mockFS := &mockEmbedFS{
			dirs: map[string][]os.DirEntry{
				"test": {mockDir("foo.txt", false)},
			},
			files: map[string][]byte{
				"test/foo.txt": []byte("hello"),
			},
		}

		dst := t.TempDir()
		requireMkdirAll(t, filepath.Join(dst, "foo.txt"), 0o755)

		err := copyEmbeddedDirRecursive(mockFS, "test", dst)
		requireError(t, err)
		assertContains(t, err.Error(), "write file")
	})
}

type mockEmbedFS struct {
	files       map[string][]byte
	dirs        map[string][]os.DirEntry
	readDirErr  error
	readFileErr error
}

func (m *mockEmbedFS) ReadDir(name string) ([]os.DirEntry, error) {
	if m.readDirErr != nil {
		return nil, m.readDirErr
	}

	entries, ok := m.dirs[name]
	if !ok {
		return nil, fs.ErrNotExist
	}

	result := make([]os.DirEntry, len(entries))
	for i, e := range entries {
		if de, ok := e.(*mockDirEntry); ok {
			result[i] = de
		}
	}

	return result, nil
}

func (m *mockEmbedFS) ReadFile(name string) ([]byte, error) {
	if m.readFileErr != nil {
		return nil, m.readFileErr
	}

	data, ok := m.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}

	return data, nil
}

type mockDirEntry struct {
	name  string
	isDir bool
}

func (m *mockDirEntry) Name() string { return m.name }

func (m *mockDirEntry) IsDir() bool { return m.isDir }

func (m *mockDirEntry) Type() fs.FileMode { return 0 }

func (m *mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func mockDir(name string, isDir bool) *mockDirEntry {
	return &mockDirEntry{name: name, isDir: isDir}
}

func TestGetBuildHandlerWithConfig_MissingBuildDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	config := InertiaConfig{
		PublicPath: "public",
		PublicFS:   os.DirFS(dir),
	}

	handler := GetBuildHandlerWithConfig(config)
	requireNotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/build/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	requireEqual(t, http.StatusNotFound, rec.Code)
}

func TestGetBuildHandlerWithConfig_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	buildDir := filepath.Join(dir, "public", "build")
	requireMkdirAll(t, buildDir, 0o755)
	requireWriteFile(t, filepath.Join(buildDir, "app.js"), []byte("console.log('hello')"), 0o644)

	config := InertiaConfig{
		PublicPath: "public",
		PublicFS:   os.DirFS(dir),
	}

	handler := GetBuildHandlerWithConfig(config)
	requireNotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/build/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	requireEqual(t, http.StatusOK, rec.Code)
}

func TestInitInertia_Dev(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")
	resourcesDir := filepath.Join(dir, "resources")

	requireMkdirAll(t, publicDir, 0o755)
	requireWriteFile(t, filepath.Join(publicDir, "hot"), []byte("http://localhost:5173\n"), 0o644)
	requireMkdirAll(t, filepath.Join(resourcesDir, "views"), 0o755)
	requireWriteFile(t, filepath.Join(resourcesDir, "views", "root.html"), []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	config := InertiaConfig{
		InertiaEnv:    EnvDev,
		PublicPath:    "public",
		ResourcesPath: "resources",
		PublicFS:      os.DirFS(dir),
		ResourcesFS:   os.DirFS(dir),
	}

	i, err := InitInertia(config)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestInitInertia_Prod(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")
	resourcesDir := filepath.Join(dir, "resources")

	requireMkdirAll(t, filepath.Join(publicDir, "build"), 0o755)
	requireWriteFile(t, filepath.Join(publicDir, "build", "manifest.json"), []byte(`{"resources/js/app.jsx":{"file":"assets/app.js","src":"resources/js/app.jsx"}}`), 0o644)
	requireMkdirAll(t, filepath.Join(resourcesDir, "views"), 0o755)
	requireWriteFile(t, filepath.Join(resourcesDir, "views", "root.html"), []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	config := InertiaConfig{
		InertiaEnv:    EnvProd,
		PublicPath:    "public",
		ResourcesPath: "resources",
		PublicFS:      os.DirFS(dir),
		ResourcesFS:   os.DirFS(dir),
	}

	i, err := InitInertia(config)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestInitInertia_AutoDevFromHotFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")
	resourcesDir := filepath.Join(dir, "resources")

	requireMkdirAll(t, publicDir, 0o755)
	requireWriteFile(t, filepath.Join(publicDir, "hot"), []byte("http://localhost:5173\n"), 0o644)
	requireMkdirAll(t, filepath.Join(resourcesDir, "views"), 0o755)
	requireWriteFile(t, filepath.Join(resourcesDir, "views", "root.html"), []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	config := InertiaConfig{
		InertiaEnv:    EnvProd,
		PublicPath:    "public",
		ResourcesPath: "resources",
		PublicFS:      os.DirFS(dir),
		ResourcesFS:   os.DirFS(dir),
	}

	i, err := InitInertia(config)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestInitInertia_MissingRootView(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")

	requireMkdirAll(t, filepath.Join(publicDir, "build"), 0o755)
	requireWriteFile(t, filepath.Join(publicDir, "build", "manifest.json"), []byte(`{}`), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	config := InertiaConfig{
		InertiaEnv:    EnvProd,
		PublicPath:    "public",
		ResourcesPath: "resources",
		PublicFS:      os.DirFS(dir),
		ResourcesFS:   os.DirFS(dir),
	}

	i, err := InitInertia(config)
	requireError(t, err)
	requireNil(t, i)
}

func TestOSProcessRunner_Start_WaitError(t *testing.T) {
	t.Parallel()
	runner := newOSProcessRunner(ProcessConfig{
		Name: "echo",
		Args: []string{"hello"},
	})

	ctx := context.Background()
	err := runner.Start(ctx)
	requireNoError(t, err)

	time.Sleep(200 * time.Millisecond)
}

func TestNewInertia_TempWriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	config := InertiaConfig{
		InertiaEnv:    EnvProd,
		PublicPath:    filepath.Join(dir, "public"),
		ResourcesPath: filepath.Join(dir, "resources"),
		PublicFS:      os.DirFS(dir),
		ResourcesFS:   os.DirFS(dir),
	}

	i, err := InitInertia(config)
	requireError(t, err)
	requireNil(t, i)
}

func TestEnsureScaffoldFiles_CopyResourcesError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "public"), 0o755)

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvDev,
		PublicPath:    filepath.Join(dir, "public"),
		ResourcesPath: filepath.Join(dir, "resources"),
	})

	oldFunc := getEmbeddedResourcesFS()
	setEmbeddedResourcesFS(func() embedFS {
		return &mockEmbedFS{readDirErr: errors.New("boom")}
	})
	defer func() { setEmbeddedResourcesFS(oldFunc) }()

	err := provider.ensureScaffoldFiles()
	requireError(t, err)
	assertContains(t, err.Error(), "copy resources")
}

func TestEnsureScaffoldFiles_CopyPublicError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "resources"), 0o755)

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvDev,
		PublicPath:    filepath.Join(dir, "public"),
		ResourcesPath: filepath.Join(dir, "resources"),
	})

	oldFunc := getEmbeddedPublicFS()
	setEmbeddedPublicFS(func() embedFS {
		return &mockEmbedFS{readDirErr: errors.New("boom")}
	})
	defer func() { setEmbeddedPublicFS(oldFunc) }()

	err := provider.ensureScaffoldFiles()
	requireError(t, err)
	assertContains(t, err.Error(), "copy public assets")
}

func TestProviderBoot_RuntimeServicesError(t *testing.T) {
	t.Parallel()
	oldFactory := getRunCommandFactory()
	defer func() { setRunCommandFactory(oldFactory) }()

	setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
		return nil, errors.New("mocked error")
	})

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:    true,
		Env:        EnvProd,
		SSREnabled: true,
	})
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Boot(ctx, container)
	requireError(t, err)
	assertContains(t, err.Error(), "failed to start runtime services")
}
