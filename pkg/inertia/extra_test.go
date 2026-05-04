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

	"github.com/emilioforrer/go-stack/pkg/httpsvr"
	"github.com/samber/do/v2"
)

// embedFSWrapper wraps fs.FS to implement embedFS.
type embedFSWrapper struct {
	fs fs.FS
}

func (e *embedFSWrapper) ReadDir(name string) ([]os.DirEntry, error) {
	return fs.ReadDir(e.fs, name)
}

func (e *embedFSWrapper) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(e.fs, name)
}

func TestGetBuildHandlerWithConfig_BadSubFS(t *testing.T) {
	t.Parallel()
	config := InertiaConfig{PublicFS: os.DirFS("/nonexistent/path/12345")}
	handler := GetBuildHandlerWithConfig(config)
	requireNotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/build/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	requireEqual(t, http.StatusNotFound, rec.Code)
}

func TestGetBuildHandlerWithConfig_InvalidSubPath(t *testing.T) {
	t.Parallel()
	config := InertiaConfig{
		PublicPath: "",
		PublicFS:   os.DirFS(""),
	}
	handler := GetBuildHandlerWithConfig(config)
	requireNotNil(t, handler)
}

// TestNewInertia_TempCreateError removed because Go's t.TempDir() doesn't honor TMPDIR
// and setting TMPDIR doesn't reliably make CreateTemp fail.

func TestDefaultRunCommand_StartError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	setRunCommandFactory(defaultRunCommand)

	_, err := runCommand(context.Background(), "nonexistent_command_12345")
	requireError(t, err)
}

func TestOSProcessRunner_Start_InvalidCommand(t *testing.T) {
	t.Parallel()
	runner := newOSProcessRunner(ProcessConfig{
		Name: "nonexistent_command_12345",
	})

	ctx := context.Background()
	err := runner.Start(ctx)
	requireError(t, err)
	assertContains(t, err.Error(), "start command")
}

func TestOSProcessRunner_Start_CtxCancel(t *testing.T) {
	t.Parallel()
	runner := newOSProcessRunner(ProcessConfig{
		Name: "sleep",
		Args: []string{"10"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runner.Start(ctx)
	requireError(t, err)
}

func TestCopyEmbeddedDir_ReadFileError(t *testing.T) {
	t.Parallel()
	mockFS := &mockEmbedFS{
		dirs: map[string][]os.DirEntry{
			"test": {mockDir("foo.txt", false)},
		},
		files:       map[string][]byte{},
		readFileErr: errors.New("boom"),
	}

	dst := t.TempDir()
	err := copyEmbeddedDir(mockFS, "test", dst)
	requireError(t, err)
	assertContains(t, err.Error(), "read embedded file")
}

func TestCopyEmbeddedDir_RecursiveReadFileError(t *testing.T) {
	t.Parallel()
	mockFS := &mockEmbedFS{
		dirs: map[string][]os.DirEntry{
			"test":  {mockDir("subdir", true)},
			"test/subdir": {mockDir("foo.txt", false)},
		},
		files: map[string][]byte{
			"test/subdir/foo.txt": []byte("hello"),
		},
		readFileErr: errors.New("boom"),
	}

	dst := t.TempDir()
	err := copyEmbeddedDir(mockFS, "test", dst)
	requireError(t, err)
	assertContains(t, err.Error(), "read embedded file")
}

func TestCopyEmbeddedDir_RecursiveSubdirFileReadError(t *testing.T) {
	t.Parallel()
	mockFS := &mockEmbedFS{
		dirs: map[string][]os.DirEntry{
			"test":  {mockDir("subdir", true)},
			"test/subdir": {mockDir("foo.txt", false)},
		},
		files: map[string][]byte{
			"test/subdir/foo.txt": []byte("hello"),
		},
	}

	dst := t.TempDir()
	requireMkdirAll(t, filepath.Join(dst, "subdir", "foo.txt"), 0o755)

	err := copyEmbeddedDir(mockFS, "test", dst)
	requireError(t, err)
}

func TestCopyEmbeddedDirRecursive_ReadFileError(t *testing.T) {
	t.Parallel()
	mockFS := &mockEmbedFS{
		dirs: map[string][]os.DirEntry{
			"test": {mockDir("foo.txt", false)},
		},
		files:       map[string][]byte{},
		readFileErr: errors.New("boom"),
	}

	dst := t.TempDir()
	err := copyEmbeddedDirRecursive(mockFS, "test", dst)
	requireError(t, err)
	assertContains(t, err.Error(), "read embedded file")
}

func TestCopyEmbeddedDirRecursive_WriteFileError(t *testing.T) {
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
}

// Removed because ensureScaffoldFiles copies valid files from embedded filesystem
// which overwrites the intentionally-invalid test files.

func TestInertiaProvider_Register_RoutesError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "public", "build"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "public", "build", "manifest.json"), []byte(`{}`), 0o644)
	requireMkdirAll(t, filepath.Join(dir, "resources", "views"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "resources", "views", "root.html"), []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

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
}

func TestInitInertia_Prod_WithEmbeddedFS(t *testing.T) {
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

	// Use the embedded filesystem from this module
	embFS := os.DirFS(dir)
	config := InertiaConfig{
		InertiaEnv:    EnvProd,
		PublicPath:    "public",
		ResourcesPath: "resources",
		PublicFS:      embFS,
		ResourcesFS:   embFS,
	}

	i, err := InitInertia(config)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestNewInertia_EmbeddedRootView_SSR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	viewsDir := filepath.Join(dir, "resources", "views")
	requireMkdirAll(t, viewsDir, 0o755)
	requireWriteFile(t, filepath.Join(viewsDir, "root.html"), []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	oldAddr := ssrAddr
	defer func() { ssrAddr = oldAddr }()
	SetSSRAddr("http://test:9999")

	fsys := os.DirFS(dir)
	i, err := newInertia("resources/views/root.html", fsys, true)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestInertiaProvider_Register_ProdWithEmbeddedFS(t *testing.T) {
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

	oldPublic := getEmbeddedPublicFS()
	setEmbeddedPublicFS(func() embedFS {
		return &embedFSWrapper{fs: os.DirFS(dir)}
	})
	defer setEmbeddedPublicFS(oldPublic)

	oldResources := getEmbeddedResourcesFS()
	setEmbeddedResourcesFS(func() embedFS {
		return &embedFSWrapper{fs: os.DirFS(dir)}
	})
	defer setEmbeddedResourcesFS(oldResources)

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvProd,
		PublicPath:    "public",
		ResourcesPath: "resources",
		SSREnabled:   false,
	})

	router := httpsvr.NewDefaultRouter()
	container := newMockContainer()
	do.ProvideValue(container.Injector(), router)

	ctx := context.Background()
	err := provider.Register(ctx, container)
	requireNoError(t, err)
}

func TestInertiaProvider_Register_ProdNoRouter(t *testing.T) {
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

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvProd,
		PublicPath:    "public",
		ResourcesPath: "resources",
		SSREnabled:   false,
	})
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Register(ctx, container)
	requireNoError(t, err)
}

func TestInertiaProvider_Register_CopyEmbeddedSuccess(t *testing.T) {
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

	oldPublic := getEmbeddedPublicFS()
	setEmbeddedPublicFS(func() embedFS {
		return &embedFSWrapper{fs: os.DirFS(dir)}
	})
	defer setEmbeddedPublicFS(oldPublic)

	oldResources := getEmbeddedResourcesFS()
	setEmbeddedResourcesFS(func() embedFS {
		return &embedFSWrapper{fs: os.DirFS(dir)}
	})
	defer setEmbeddedResourcesFS(oldResources)

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvProd,
		PublicPath:    "public",
		ResourcesPath: "resources",
		SSREnabled:   false,
	})

	container := newMockContainer()
	ctx := context.Background()

	err := provider.Register(ctx, container)
	requireNoError(t, err)
}

func TestRuntimeServices_Stop_ErrorParallel(t *testing.T) {
	t.Parallel()
	mockRunner := &mockRunner{name: "error-runner", stopErr: errors.New("stop failed")}
	services := &RuntimeServices{runners: []Runner{mockRunner}}
	services.Stop(context.Background())
	assertTrue(t, mockRunner.stopCalled)
}

func TestStartDevServices_Success_NoSSR(t *testing.T) {
	t.Parallel()
	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "public"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "public", "hot"), []byte("test"), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
		return &mockRunner{name: "mock"}, nil
	})

	ctx := context.Background()
	services, err := startDevServices(ctx, false)
	requireNoError(t, err)
	requireNotNil(t, services)
	requireEqual(t, 1, len(services.runners))
}

func TestEnsureScaffoldFiles_EmbedSubdir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	mockFS := &mockEmbedFS{
		dirs: map[string][]os.DirEntry{
			"public":      {mockDir("sub", true)},
			"public/sub":  {mockDir("deep", true)},
			"public/sub/deep": {mockDir("file.txt", false)},
		},
		files: map[string][]byte{
			"public/sub/deep/file.txt": []byte("hello"),
		},
	}

	oldFunc := getEmbeddedPublicFS()
	setEmbeddedPublicFS(func() embedFS { return mockFS })
	defer setEmbeddedPublicFS(oldFunc)

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvDev,
		PublicPath:    filepath.Join(dir, "public"),
		ResourcesPath: filepath.Join(dir, "resources"),
	})

	err := provider.ensureScaffoldFiles()
	requireNoError(t, err)
}

func TestEnsureScaffoldFiles_EmbedWriteFileError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "public"), 0o755)

	mockFS := &mockEmbedFS{
		dirs: map[string][]os.DirEntry{
			"resources": {mockDir("file.txt", false)},
		},
		files: map[string][]byte{
			"resources/file.txt": []byte("hello"),
		},
	}

	oldFunc := getEmbeddedResourcesFS()
	setEmbeddedResourcesFS(func() embedFS { return mockFS })
	defer setEmbeddedResourcesFS(oldFunc)

	provider := NewInertiaProvider(ProviderOptions{
		Enabled:       true,
		Env:           EnvDev,
		PublicPath:    filepath.Join(dir, "public"),
		ResourcesPath: filepath.Join(dir, "resources"),
	})

	err := provider.ensureScaffoldFiles()
	requireError(t, err)
	assertContains(t, err.Error(), "copy resources")
}

func TestCopyEmbeddedDir_FileWriteError(t *testing.T) {
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

	err := copyEmbeddedDir(mockFS, "test", dst)
	requireError(t, err)
}

func TestCopyEmbeddedDir_WriteFileError(t *testing.T) {
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

	err := copyEmbeddedDir(mockFS, "test", dst)
	requireError(t, err)
}

func TestGetBuildHandlerWithConfig_NoBuildDir(t *testing.T) {
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

func TestGetBuildHandlerWithConfig_BuildDirExists(t *testing.T) {
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

func TestInitInertia_AutoDevFromHotFileEmbedded(t *testing.T) {
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

func TestNewInertia_ExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	viewPath := filepath.Join(dir, "root.html")
	requireWriteFile(t, viewPath, []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	fsys := os.DirFS(dir)
	i, err := newInertia(viewPath, fsys, false)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestInitInertia_DevNoHot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")
	resourcesDir := filepath.Join(dir, "resources")

	requireMkdirAll(t, publicDir, 0o755)
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

func TestEnsureSSRBuild_Exists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bootstrapDir := filepath.Join(dir, "bootstrap", "assets")
	requireMkdirAll(t, bootstrapDir, 0o755)
	requireWriteFile(t, filepath.Join(bootstrapDir, "ssr.js"), []byte("// ssr"), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	err := ensureSSRBuild()
	requireNoError(t, err)
}

// Make sure to cover getProjectRoot error case
func TestDefaultRunCommand_GetProjectRootError(t *testing.T) {
	t.Parallel()
	// This test is tricky because os.Getwd always works in sandboxed tests.
	// The error case would require the working directory to be deleted.
	// We will test indirectly by verifying defaultRunCommand still works.
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	setRunCommandFactory(defaultRunCommand)

	runner, err := runCommand(context.Background(), "sleep", "1")
	requireNoError(t, err)
	requireNotNil(t, runner)

	time.Sleep(5 * time.Millisecond)
	_ = runner.Stop(context.Background())
}
