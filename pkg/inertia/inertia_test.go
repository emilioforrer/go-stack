package inertia

import (
	"context"
	"embed"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

//go:embed testdata/hot.txt
var testdataHot []byte

//go:embed testdata/manifest.json
var testdataManifest []byte

func TestNormalizeInertiaEnv(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty defaults to dev", "", EnvDev},
		{"lowercase dev", "dev", EnvDev},
		{"uppercase DEV", "DEV", EnvDev},
		{"mixed case Dev", "Dev", EnvDev},
		{"lowercase prod", "prod", EnvProd},
		{"uppercase PROD", "PROD", EnvProd},
		{"trim spaces", "  dev  ", EnvDev},
		{"unknown returns as-is", "staging", "staging"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeInertiaEnv(tt.input)
			requireEqual(t, tt.want, got)
		})
	}
}

func TestNormalizePublicPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty defaults to public", "", defaultPublicPath},
		{"custom path", "my/public", "my/public"},
		{"trims slashes", "/public/", "public"},
		{"trims spaces", "  public  ", "public"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizePublicPath(tt.input)
			requireEqual(t, tt.want, got)
		})
	}
}

func TestNormalizeResourcesPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty defaults to resources", "", defaultResourcesPath},
		{"custom path", "my/resources", "my/resources"},
		{"trims slashes", "/resources/", "resources"},
		{"trims spaces", "  resources  ", "resources"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeResourcesPath(tt.input)
			requireEqual(t, tt.want, got)
		})
	}
}

func TestResolvePublicFS(t *testing.T) {
	t.Parallel()
	t.Run("with custom fs", func(t *testing.T) {
		t.Parallel()
		var customFS embed.FS
		config := InertiaConfig{PublicFS: customFS}
		got := resolvePublicFS(config)
		requireNotNil(t, got)
	})

	t.Run("without custom fs", func(t *testing.T) {
		t.Parallel()
		config := InertiaConfig{}
		got := resolvePublicFS(config)
		requireNotNil(t, got)
	})
}

func TestResolveResourcesFS(t *testing.T) {
	t.Parallel()
	t.Run("with custom fs", func(t *testing.T) {
		t.Parallel()
		var customFS embed.FS
		config := InertiaConfig{ResourcesFS: customFS}
		got := resolveResourcesFS(config)
		requireNotNil(t, got)
	})

	t.Run("without custom fs", func(t *testing.T) {
		t.Parallel()
		config := InertiaConfig{}
		got := resolveResourcesFS(config)
		requireNotNil(t, got)
	})
}

func TestHasViteHotFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hotFile := filepath.Join(dir, "hot")

	t.Run("file exists", func(t *testing.T) {
		t.Parallel()
		requireWriteFile(t, hotFile, []byte("test"), 0o644)
		defer os.Remove(hotFile)

		fsys := os.DirFS(dir)
		assertTrue(t, hasViteHotFile("hot", fsys))
	})

	t.Run("file does not exist", func(t *testing.T) {
		t.Parallel()
		fsys := os.DirFS(dir)
		assertFalse(t, hasViteHotFile("nonexistent", fsys))
	})
}

func TestViteHotTemplateFunc(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hotFile := filepath.Join(dir, "hot")
	requireWriteFile(t, hotFile, []byte("http://localhost:5173\n"), 0o644)

	fsys := os.DirFS(dir)
	fn := viteHotTemplateFunc("hot", fsys)

	t.Run("valid entry", func(t *testing.T) {
		t.Parallel()
		got, err := fn("resources/js/app.jsx")
		requireNoError(t, err)
		requireEqual(t, "//localhost:5173/resources/js/app.jsx", got)
	})

	t.Run("entry with leading slash", func(t *testing.T) {
		t.Parallel()
		got, err := fn("/resources/js/app.jsx")
		requireNoError(t, err)
		requireEqual(t, "//localhost:5173/resources/js/app.jsx", got)
	})

	t.Run("empty entry", func(t *testing.T) {
		t.Parallel()
		got, err := fn("")
		requireNoError(t, err)
		requireEqual(t, "//localhost:5173", got)
	})

	t.Run("missing hot file", func(t *testing.T) {
		t.Parallel()
		badFn := viteHotTemplateFunc("missing", fsys)
		_, err := badFn("test")
		requireError(t, err)
	})
}

func TestNormalizeHotURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"http url", "http://localhost:5173", "//localhost:5173"},
		{"https url", "https://localhost:5173", "//localhost:5173"},
		{"empty defaults to localhost", "", "//localhost:8000"},
		{"other format", "custom", "//localhost:8000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeHotURL(tt.input)
			requireEqual(t, tt.want, got)
		})
	}
}

func TestParseManifest(t *testing.T) {
	t.Parallel()

	t.Run("valid manifest", func(t *testing.T) {
		t.Parallel()
		data := []byte(`{"resources/js/app.jsx":{"file":"assets/app.js","src":"resources/js/app.jsx"}}`)
		assets, err := parseManifest(data)
		requireNoError(t, err)
		assertLen(t, assets, 1)
		requireEqual(t, "assets/app.js", assets["resources/js/app.jsx"].File)
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		data := []byte(`{invalid`)
		_, err := parseManifest(data)
		requireError(t, err)
	})
}

func TestViteProdFunc(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	manifestFile := filepath.Join(dir, "manifest.json")
	requireWriteFile(t, manifestFile, []byte(`{
		"resources/js/app.jsx": {"file": "assets/app.js", "src": "resources/js/app.jsx"}
	}`), 0o644)

	fsys := os.DirFS(dir)
	fn := viteProdFunc(fsys, "manifest.json", "/build/")
	requireNotNil(t, fn)

	t.Run("existing asset", func(t *testing.T) {
		t.Parallel()
		got, err := fn("resources/js/app.jsx")
		requireNoError(t, err)
		requireEqual(t, "/build/assets/app.js", got)
	})

	t.Run("missing asset", func(t *testing.T) {
		t.Parallel()
		_, err := fn("missing")
		requireError(t, err)
		assertErrorIs(t, err, ErrAssetNotFound)
	})

	t.Run("missing manifest", func(t *testing.T) {
		t.Parallel()
		badFn := viteProdFunc(fsys, "missing.json", "/build/")
		requireNil(t, badFn)
	})
}

func TestEnsureViteManifest(t *testing.T) {
	t.Parallel()

	t.Run("manifest already exists", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "manifest.json")
		requireWriteFile(t, manifestPath, []byte("{}"), 0o644)

		fsys := os.DirFS(dir)
		err := ensureViteManifest(fsys, dir, "manifest.json", true)
		requireNoError(t, err)
	})

	t.Run("move from .vite directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		viteDir := filepath.Join(dir, "build", ".vite")
		requireMkdirAll(t, viteDir, 0o755)
		requireWriteFile(t, filepath.Join(viteDir, "manifest.json"), []byte("{}"), 0o644)

		oldWd, _ := os.Getwd()
		requireChdir(t, dir)
		defer func() { _ = os.Chdir(oldWd) }()

		fsys := os.DirFS(dir)
		err := ensureViteManifest(fsys, ".", "build/manifest.json", true)
		requireNoError(t, err)

		_, statErr := os.Stat(filepath.Join(dir, "build", "manifest.json"))
		requireNoError(t, statErr)
	})

	t.Run("embedded fs missing manifest", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fsys := os.DirFS(dir)
		err := ensureViteManifest(fsys, dir, "manifest.json", false)
		requireError(t, err)
	})
}

func TestEnsureSSRBuild(t *testing.T) {
	t.Parallel()

	t.Run("ssr bundle exists", func(t *testing.T) {
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
	})

	t.Run("ssr bundle missing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		oldWd, _ := os.Getwd()
		requireChdir(t, dir)
		defer func() { _ = os.Chdir(oldWd) }()

		err := ensureSSRBuild()
		requireError(t, err)
	})
}

func TestWaitForFile(t *testing.T) {
	t.Parallel()

	t.Run("file already exists", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		requireWriteFile(t, testFile, []byte("test"), 0o644)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := waitForFile(ctx, testFile, 50*time.Millisecond)
		requireNoError(t, err)
	})

	t.Run("context cancelled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := waitForFile(ctx, "/nonexistent/file", 100*time.Millisecond)
		requireError(t, err)
		assertErrorIs(t, err, context.Canceled)
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		err := waitForFile(ctx, "/nonexistent/file", 50*time.Millisecond)
		requireError(t, err)
		assertContains(t, err.Error(), "timeout")
	})
}

func TestRuntimeServices_Stop(t *testing.T) {
	t.Parallel()

	t.Run("stop with no runners", func(t *testing.T) {
		t.Parallel()
		services := &RuntimeServices{}
		services.Stop(context.Background()) // should not panic
	})

	t.Run("stop with runner", func(t *testing.T) {
		t.Parallel()
		mockRunner := &mockRunner{name: "test"}
		services := &RuntimeServices{runners: []Runner{mockRunner}}
		services.Stop(context.Background())
		assertTrue(t, mockRunner.stopCalled)
	})
}

type mockRunner struct {
	name       string
	startErr   error
	stopErr    error
	stopCalled bool
}

func (m *mockRunner) Start(_ context.Context) error {
	return m.startErr
}

func (m *mockRunner) Stop(_ context.Context) error {
	m.stopCalled = true
	return m.stopErr
}

func (m *mockRunner) Name() string {
	return m.name
}

func TestGetBuildHandlerWithConfig(t *testing.T) {
	t.Parallel()

	t.Run("valid build directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		buildDir := filepath.Join(dir, "public", "build")
		requireMkdirAll(t, buildDir, 0o755)
		requireWriteFile(t, filepath.Join(buildDir, "app.js"), []byte("console.log('app')"), 0o644)

		config := InertiaConfig{
			PublicPath: "public",
			PublicFS:   os.DirFS(dir),
		}

		handler := GetBuildHandlerWithConfig(config)
		requireNotNil(t, handler)
	})

	t.Run("missing build directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		config := InertiaConfig{
			PublicPath: "public",
			PublicFS:   os.DirFS(dir),
		}

		handler := GetBuildHandlerWithConfig(config)
		requireNotNil(t, handler)
	})
}

func TestGetBuildHandler(t *testing.T) {
	t.Parallel()
	requireNotNil(t, GetBuildHandler())
}

func TestStartRuntimeServices(t *testing.T) {
	t.Parallel()

	t.Run("unsupported env", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		_, err := StartRuntimeServices(ctx, RuntimeConfig{InertiaEnv: "invalid"})
		requireError(t, err)
		assertContains(t, err.Error(), "unsupported")
	})

	t.Run("prod without ssr", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		services, err := StartRuntimeServices(ctx, RuntimeConfig{InertiaEnv: EnvProd, SSREnabled: false})
		requireNoError(t, err)
		requireNotNil(t, services)
	})
}

func TestNewInertia(t *testing.T) {
	t.Parallel()

	t.Run("with filesystem root view", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		viewPath := filepath.Join(dir, "root.html")
		requireWriteFile(t, viewPath, []byte("<!DOCTYPE html><html></html>"), 0o644)

		fsys := os.DirFS(dir)
		i, err := newInertia(viewPath, fsys, false)
		requireNoError(t, err)
		requireNotNil(t, i)
	})

	t.Run("with embedded root view", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		viewPath := filepath.Join(dir, "resources", "views", "root.html")
		fsys := os.DirFS(dir)

		i, err := newInertia(viewPath, fsys, false)
		requireError(t, err)
		requireNil(t, i)
	})

	t.Run("set SSR addr", func(t *testing.T) {
		t.Parallel()
		oldAddr := ssrAddr
		defer func() { ssrAddr = oldAddr }()

		SetSSRAddr("http://test:9999")
		httpAddr := ssrAddr
		requireEqual(t, "http://test:9999", httpAddr)
	})
}

func TestEnsureViteManifest_MoveFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fsys := os.DirFS(dir)
	err := ensureViteManifest(fsys, dir, "manifest.json", true)
	requireError(t, err)
	assertContains(t, err.Error(), "failed to move")
}

func TestViteProdFunc_ParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	manifestFile := filepath.Join(dir, "manifest.json")
	requireWriteFile(t, manifestFile, []byte(`not json`), 0o644)

	fsys := os.DirFS(dir)
	fn := viteProdFunc(fsys, "manifest.json", "/build/")
	requireNil(t, fn)
}

func TestRunClientDevServer(t *testing.T) {
	t.Run("returns error when factory fails", func(t *testing.T) {
		oldFactory := getRunCommandFactory()
		defer func() { setRunCommandFactory(oldFactory) }()

		setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
			return nil, errors.New("mocked error")
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := runClientDevServer(ctx)
		requireError(t, err)
		assertContains(t, err.Error(), "mocked error")
	})
}

func TestRunSSRBuildWatch(t *testing.T) {
	t.Run("returns error when factory fails", func(t *testing.T) {
		oldFactory := getRunCommandFactory()
		defer func() { setRunCommandFactory(oldFactory) }()

		setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
			return nil, errors.New("mocked error")
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := runSSRBuildWatch(ctx)
		requireError(t, err)
		assertContains(t, err.Error(), "mocked error")
	})
}

func TestRunSSR(t *testing.T) {
	t.Run("returns error when factory fails", func(t *testing.T) {
		oldFactory := getRunCommandFactory()
		defer func() { setRunCommandFactory(oldFactory) }()

		setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
			return nil, errors.New("mocked error")
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := runSSR(ctx)
		requireError(t, err)
		assertContains(t, err.Error(), "mocked error")
	})
}

func TestOSProcessRunner(t *testing.T) {
	t.Parallel()

	t.Run("start and stop", func(t *testing.T) {
		t.Parallel()
		config := ProcessConfig{
			Name: "sleep",
			Args: []string{"10"},
		}
		runner := newOSProcessRunner(config)
		requireNotNil(t, runner)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := runner.Start(ctx)
		requireNoError(t, err)

		time.Sleep(50 * time.Millisecond)

		stopErr := runner.Stop(ctx)
		requireNoError(t, stopErr)
	})

	t.Run("start when already started", func(t *testing.T) {
		t.Parallel()
		config := ProcessConfig{
			Name: "sleep",
			Args: []string{"10"},
		}
		runner := newOSProcessRunner(config)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_ = runner.Start(ctx)
		err := runner.Start(ctx)
		requireError(t, err)
		assertContains(t, err.Error(), "already started")

		_ = runner.Stop(ctx)
	})

	t.Run("stop without start", func(t *testing.T) {
		t.Parallel()
		config := ProcessConfig{
			Name: "sleep",
			Args: []string{"10"},
		}
		runner := newOSProcessRunner(config)

		err := runner.Stop(context.Background())
		requireNoError(t, err)
	})

	t.Run("name", func(t *testing.T) {
		t.Parallel()
		config := ProcessConfig{Name: "test"}
		runner := newOSProcessRunner(config)
		requireEqual(t, "test", runner.Name())
	})
}

func TestNormalizeHotURLHttps(t *testing.T) {
	t.Parallel()
	url := normalizeHotURL("https://example.com:8080")
	requireEqual(t, "//example.com:8080", url)
}

func TestNewInertia_EmbeddedRootView(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	viewsDir := filepath.Join(dir, "resources", "views")
	requireMkdirAll(t, viewsDir, 0o755)
	requireWriteFile(t, filepath.Join(viewsDir, "root.html"), []byte(`<!DOCTYPE html><html></html>`), 0o644)

	fsys := os.DirFS(dir)
	i, err := newInertia("resources/views/root.html", fsys, false)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestStartProdServices_WithSSRError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	ctx := context.Background()
	_, err := startProdServices(ctx, true)
	requireError(t, err)
}

func TestEnsureSSRBuild_Missing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	err := ensureSSRBuild()
	requireError(t, err)
}

func TestResolvePublicFS_ReturnsDirFS(t *testing.T) {
	t.Parallel()
	config := InertiaConfig{}
	fsys := resolvePublicFS(config)
	requireNotNil(t, fsys)
}

func TestResolveResourcesFS_ReturnsDirFS(t *testing.T) {
	t.Parallel()
	config := InertiaConfig{}
	fsys := resolveResourcesFS(config)
	requireNotNil(t, fsys)
}

func TestHasViteHotFile_NotExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fsys := os.DirFS(dir)
	assertFalse(t, hasViteHotFile("does_not_exist", fsys))
}

func TestViteProdFunc_MissingManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fsys := os.DirFS(dir)
	fn := viteProdFunc(fsys, "no_manifest.json", "/build/")
	requireNil(t, fn)
}

func TestViteHotTemplateFunc_Integration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		hotContent  string
		entry       string
		expectedURL string
		expectErr   bool
	}{
		{
			name:        "standard vite dev server",
			hotContent:  "http://localhost:5173\n",
			entry:       "resources/js/app.jsx",
			expectedURL: "//localhost:5173/resources/js/app.jsx",
		},
		{
			name:        "empty entry",
			hotContent:  "https://localhost:5173",
			entry:       "",
			expectedURL: "//localhost:5173",
		},
		{
			name:      "missing hot file",
			hotContent: "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tt.hotContent != "" || !tt.expectErr {
				hotFile := filepath.Join(dir, "hot")
				requireWriteFile(t, hotFile, []byte(tt.hotContent), 0o644)
			}

			fsys := os.DirFS(dir)
			fn := viteHotTemplateFunc("hot", fsys)

			if tt.expectErr {
				_, err := fn("test")
				requireError(t, err)
				return
			}

			got, err := fn(tt.entry)
			requireNoError(t, err)
			requireEqual(t, tt.expectedURL, got)
		})
	}
}

func TestViteProdFunc_NilReturn(t *testing.T) {
	t.Parallel()

	t.Run("manifest read error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fsys := os.DirFS(dir)
		fn := viteProdFunc(fsys, "not_found.json", "/build/")
		requireNil(t, fn)
	})

	t.Run("manifest parse error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "manifest.json")
		requireWriteFile(t, manifestPath, []byte("not json"), 0o644)
		fsys := os.DirFS(dir)
		fn := viteProdFunc(fsys, "manifest.json", "/build/")
		requireNil(t, fn)
	})

	t.Run("successful parse", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "manifest.json")
		requireWriteFile(t, manifestPath, []byte(`{
			"resources/js/app.jsx": {"file": "assets/app-abc123.js", "src": "resources/js/app.jsx"}
		}`), 0o644)
		fsys := os.DirFS(dir)
		fn := viteProdFunc(fsys, "manifest.json", "/build/")
		requireNotNil(t, fn)

		result, err := fn("resources/js/app.jsx")
		requireNoError(t, err)
		requireEqual(t, "/build/assets/app-abc123.js", result)
	})
}

func TestRuntimeServices_Stop_Error(t *testing.T) {
	t.Parallel()
	mockRunner := &mockRunner{name: "error-runner", stopErr: errors.New("stop failed")}
	services := &RuntimeServices{runners: []Runner{mockRunner}}
	services.Stop(context.Background())
	assertTrue(t, mockRunner.stopCalled)
}

func TestStartRuntimeServices_Dev(t *testing.T) {
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
		return &mockRunner{name: "dev"}, nil
	})

	ctx := context.Background()
	services, err := StartRuntimeServices(ctx, RuntimeConfig{InertiaEnv: EnvDev, SSREnabled: false})
	requireNoError(t, err)
	requireNotNil(t, services)
}

func TestStartRuntimeServices_Dev_WithSSR(t *testing.T) {
	t.Parallel()
	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "public"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "public", "hot"), []byte("test"), 0o644)
	requireMkdirAll(t, filepath.Join(dir, "bootstrap", "assets"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "bootstrap", "assets", "ssr.js"), []byte("// ssr"), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
		return &mockRunner{name: "mock"}, nil
	})

	ctx := context.Background()
	services, err := StartRuntimeServices(ctx, RuntimeConfig{InertiaEnv: EnvDev, SSREnabled: true})
	requireNoError(t, err)
	requireNotNil(t, services)
}

func TestInitInertia_Dev_WithSSR(t *testing.T) {
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

	oldAddr := ssrAddr
	defer func() { ssrAddr = oldAddr }()
	SetSSRAddr("http://test:9999")

	config := InertiaConfig{
		InertiaEnv:    EnvDev,
		PublicPath:    "public",
		ResourcesPath: "resources",
		PublicFS:      os.DirFS(dir),
		ResourcesFS:   os.DirFS(dir),
		SSREnabled:    true,
	}

	i, err := InitInertia(config)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestInitInertia_Dev_MissingRootView(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")

	requireMkdirAll(t, publicDir, 0o755)
	requireWriteFile(t, filepath.Join(publicDir, "hot"), []byte("http://localhost:5173\n"), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	config := InertiaConfig{
		InertiaEnv:    EnvDev,
		PublicPath:    "public",
		ResourcesPath: "resources",
		PublicFS:      os.DirFS(dir),
		ResourcesFS:   os.DirFS(dir),
		SSREnabled:    false,
	}

	_, err := InitInertia(config)
	requireError(t, err)
}

func TestInitInertia_Prod_ManifestError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "public")
	resourcesDir := filepath.Join(dir, "resources")

	requireMkdirAll(t, filepath.Join(publicDir, "build"), 0o755)
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
		SSREnabled:    false,
	}

	i, err := InitInertia(config)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestNewInertia_WithSSR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	viewPath := filepath.Join(dir, "root.html")
	requireWriteFile(t, viewPath, []byte("<!DOCTYPE html><html></html>"), 0o644)

	fsys := os.DirFS(dir)
	oldAddr := ssrAddr
	defer func() { ssrAddr = oldAddr }()
	SetSSRAddr("http://test:9999")

	i, err := newInertia(viewPath, fsys, true)
	requireNoError(t, err)
	requireNotNil(t, i)
}

func TestDefaultRunCommand(t *testing.T) {
	t.Parallel()
	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	setRunCommandFactory(defaultRunCommand)

	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	runner, err := runCommand(context.Background(), "sleep", "10")
	requireNoError(t, err)
	requireNotNil(t, runner)

	time.Sleep(10 * time.Millisecond)

	err = runner.Stop(context.Background())
	requireNoError(t, err)
}

func TestGetProjectRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	root, err := getProjectRoot()
	requireNoError(t, err)
	if root == "" {
		t.Fatal("expected non-empty project root")
	}
}

func TestNewInertia_InvalidRootView(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	viewPath := filepath.Join(dir, "root.html")
	requireWriteFile(t, viewPath, []byte(`<!DOCTYPE html><html>{{ .inertia }}</html>`), 0o644)

	fsys := os.DirFS(dir)
	_, err := newInertia("nonexistent/root.html", fsys, false)
	requireError(t, err)
	assertContains(t, err.Error(), "failed to read root view")
}


func TestStartDevServices_Success(t *testing.T) {
	t.Parallel()
	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "public"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "public", "hot"), []byte("test"), 0o644)
	requireMkdirAll(t, filepath.Join(dir, "bootstrap", "assets"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "bootstrap", "assets", "ssr.js"), []byte("// ssr"), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
		return &mockRunner{name: "mock"}, nil
	})

	ctx := context.Background()
	services, err := startDevServices(ctx, true)
	requireNoError(t, err)
	requireNotNil(t, services)
	requireEqual(t, 3, len(services.runners))
}

func TestStartDevServices_HotTimeout(t *testing.T) {
	t.Parallel()
	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
		return &mockRunner{name: "dev"}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := startDevServices(ctx, false)
	requireError(t, err)
	assertContains(t, err.Error(), "wait for hot file")
}

func TestStartDevServices_SSRBuildError(t *testing.T) {
	t.Parallel()
	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "public"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "public", "hot"), []byte("test"), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	setRunCommandFactory(func(_ context.Context, name string, args ...string) (Runner, error) {
		if len(args) >= 2 && args[1] == "dev:ssr:build" {
			return nil, errors.New("ssr build error")
		}
		return &mockRunner{name: name}, nil
	})

	ctx := context.Background()
	_, err := startDevServices(ctx, true)
	requireError(t, err)
	assertContains(t, err.Error(), "ssr build watch")
}

func TestStartDevServices_SSRTimeout(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := startDevServices(ctx, true)
	requireError(t, err)
	assertContains(t, err.Error(), "wait for ssr bundle")
}

func TestStartDevServices_SSRError(t *testing.T) {
	t.Parallel()
	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "public"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "public", "hot"), []byte("test"), 0o644)
	requireMkdirAll(t, filepath.Join(dir, "bootstrap", "assets"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "bootstrap", "assets", "ssr.js"), []byte("// ssr"), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	setRunCommandFactory(func(_ context.Context, name string, args ...string) (Runner, error) {
		if len(args) >= 2 && args[1] == "ssr" {
			return nil, errors.New("ssr error")
		}
		return &mockRunner{name: name}, nil
	})

	ctx := context.Background()
	_, err := startDevServices(ctx, true)
	requireError(t, err)
	assertContains(t, err.Error(), "start ssr server")
}

func TestStartProdServices_WithSSR_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	requireMkdirAll(t, filepath.Join(dir, "bootstrap", "assets"), 0o755)
	requireWriteFile(t, filepath.Join(dir, "bootstrap", "assets", "ssr.js"), []byte("// ssr"), 0o644)

	oldWd, _ := os.Getwd()
	requireChdir(t, dir)
	defer func() { _ = os.Chdir(oldWd) }()

	oldFactory := getRunCommandFactory()
	defer setRunCommandFactory(oldFactory)

	setRunCommandFactory(func(_ context.Context, _ string, _ ...string) (Runner, error) {
		return &mockRunner{name: "ssr"}, nil
	})

	ctx := context.Background()
	services, err := startProdServices(ctx, true)
	requireNoError(t, err)
	requireNotNil(t, services)
	requireEqual(t, 1, len(services.runners))
}
