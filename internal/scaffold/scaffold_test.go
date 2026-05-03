// Package scaffold provides full test coverage for the scaffold package.
package scaffold

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newFakeZip(t testing.TB, files map[string]string) []byte {
	t.Helper()
	return newFakeZipWithDirs(t, files, nil)
}

func newFakeZipWithDirs(t testing.TB, files map[string]string, dirs map[string]os.FileMode) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, mode := range dirs {
		h := &zip.FileHeader{
			Name:   name,
			Method: zip.Store,
		}
		h.SetMode(mode)
		_, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatalf("failed to create zip header for dir %s: %v", name, err)
		}
	}

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		_, err = w.Write([]byte(content))
		if err != nil {
			t.Fatalf("failed to write zip content for %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return buf.Bytes()
}

func newFakeZipWithFileMode(t testing.TB, files map[string]string, mode os.FileMode) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		h := &zip.FileHeader{
			Name:   name,
			Method: zip.Store,
		}
		if mode != 0 {
			h.SetMode(mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatalf("failed to create zip header for %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content for %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return buf.Bytes()
}

type errReader struct {
	err error
}

func (r *errReader) Read([]byte) (int, error) {
	return 0, r.err
}

func (r *errReader) Close() error {
	return nil
}

func TestNewProjectWithDeps(t *testing.T) {
	t.Parallel()

	t.Run("empty target path returns error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		err := NewProjectWithDeps("", "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if err.Error() != "target path is required" {
			t.Errorf("expected 'target path is required', got: %v", err)
		}
	})

	t.Run("http get error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		deps.HTTPGet = func(string) (*http.Response, error) {
			return nil, errors.New("network error")
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "downloading zip") {
			t.Errorf("expected error to contain 'downloading zip', got: %v", err)
		}
		if !strings.Contains(err.Error(), "network error") {
			t.Errorf("expected error to contain 'network error', got: %v", err)
		}
	})

	t.Run("non-200 status code", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "unexpected status code: 404") {
			t.Errorf("expected error to contain 'unexpected status code: 404', got: %v", err)
		}
	})

	t.Run("invalid zip body", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("not a zip")),
			}, nil
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "opening zip") {
			t.Errorf("expected error to contain 'opening zip', got: %v", err)
		}
	})

	t.Run("reading zip body error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &errReader{err: errors.New("read body fail")},
			}, nil
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "reading zip body") {
			t.Errorf("expected error to contain 'reading zip body', got: %v", err)
		}
		if !strings.Contains(err.Error(), "read body fail") {
			t.Errorf("expected error to contain 'read body fail', got: %v", err)
		}
	})

	t.Run("mkdir temp error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		deps.HTTPGet = func(string) (*http.Response, error) {
			body := newFakeZip(t, map[string]string{
				"go-stack-main/internal/scaffold/template/hello.txt": "world",
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.MkdirTemp = func(string, string) (string, error) {
			return "", errors.New("mkdir temp fail")
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "creating temp dir") {
			t.Errorf("expected error to contain 'creating temp dir', got: %v", err)
		}
		if !strings.Contains(err.Error(), "mkdir temp fail") {
			t.Errorf("expected error to contain 'mkdir temp fail', got: %v", err)
		}
	})

	t.Run("mkdir all error during extraction", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/nested/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.MkdirAll = func(string, os.FileMode) error {
			return errors.New("mkdir fail")
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "creating parent directory") {
			t.Errorf("expected error to contain 'creating parent directory', got: %v", err)
		}
		if !strings.Contains(err.Error(), "mkdir fail") {
			t.Errorf("expected error to contain 'mkdir fail', got: %v", err)
		}
	})

	t.Run("create file error during extraction", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.Create = func(string) (*os.File, error) {
			return nil, errors.New("create fail")
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "creating file") {
			t.Errorf("expected error to contain 'creating file', got: %v", err)
		}
		if !strings.Contains(err.Error(), "create fail") {
			t.Errorf("expected error to contain 'create fail', got: %v", err)
		}
	})

	t.Run("open zip file error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/file.txt": "content",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.OpenZipFile = func(*zip.File) (io.ReadCloser, error) {
			return nil, errors.New("open zip file fail")
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "opening zip file") {
			t.Errorf("expected error to contain 'opening zip file', got: %v", err)
		}
		if !strings.Contains(err.Error(), "open zip file fail") {
			t.Errorf("expected error to contain 'open zip file fail', got: %v", err)
		}
	})

	t.Run("copy write error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.Create = func(name string) (*os.File, error) {
			return os.NewFile(0, name), nil // invalid file, write will fail
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "writing file") {
			t.Errorf("expected error to contain 'writing file', got: %v", err)
		}
	})

	t.Run("chmod error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZipWithFileMode(t, map[string]string{
			"go-stack-main/internal/scaffold/template/exec.sh": "#!/bin/sh",
		}, 0o755)
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.Chmod = func(string, os.FileMode) error {
			return errors.New("chmod fail")
		}
		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "setting file permissions") {
			t.Errorf("expected error to contain 'setting file permissions', got: %v", err)
		}
		if !strings.Contains(err.Error(), "chmod fail") {
			t.Errorf("expected error to contain 'chmod fail', got: %v", err)
		}
	})

	t.Run("successful extraction without name or module", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}

		err := NewProjectWithDeps(targetDir, "", "", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(content) != "world" {
			t.Errorf("expected 'world', got: %s", string(content))
		}
	})

	t.Run("successful extraction with directory entry", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZipWithDirs(t, map[string]string{
			"go-stack-main/internal/scaffold/template/sub/file.txt": "hello",
		}, map[string]os.FileMode{
			"go-stack-main/internal/scaffold/template/sub/": 0o750,
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		err := NewProjectWithDeps(targetDir, "", "", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(targetDir, "sub", "file.txt"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(content) != "hello" {
			t.Errorf("expected 'hello', got: %s", string(content))
		}
	})

	t.Run("skips non-template entries", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/LICENSE":                              "MIT",
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}

		err := NewProjectWithDeps(targetDir, "", "", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.ReadFile(filepath.Join(targetDir, "hello.txt")); err != nil {
			t.Fatalf("expected hello.txt to exist: %v", err)
		}
		if _, err := os.ReadFile(filepath.Join(targetDir, "LICENSE")); err == nil {
			t.Error("expected LICENSE to not exist")
		}
	})

	t.Run("skips exact prefix directory entry", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZipWithDirs(t, map[string]string{
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		}, map[string]os.FileMode{
			zipPrefix: 0o750,
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}

		err := NewProjectWithDeps(targetDir, "", "", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(content) != "world" {
			t.Errorf("expected 'world', got: %s", string(content))
		}
	})

	t.Run("successful extraction with name updates sonar props", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/sonar-project.properties": "sonar.projectKey=old\nsonar.projectName=old\n",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}

		err := NewProjectWithDeps(targetDir, "myapp", "", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(targetDir, "sonar-project.properties"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if !strings.Contains(string(content), "sonar.projectKey=myapp") {
			t.Errorf("expected sonar.projectKey=myapp, got: %s", string(content))
		}
		if !strings.Contains(string(content), "sonar.projectName=myapp") {
			t.Errorf("expected sonar.projectName=myapp, got: %s", string(content))
		}
	})

	t.Run("module update triggers go mod edit", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/go.mod":      "module old\n",
			"go-stack-main/internal/scaffold/template/app/main.go": "package main",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		called := false
		deps.RunGoModEdit = func(dir, mod string) error {
			called = true
			if dir != targetDir {
				t.Errorf("expected dir %s, got: %s", targetDir, dir)
			}
			if mod != "github.com/example/myapp" {
				t.Errorf("expected module github.com/example/myapp, got: %s", mod)
			}
			return nil
		}

		err := NewProjectWithDeps(targetDir, "", "github.com/example/myapp", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Error("expected RunGoModEdit to be called")
		}
	})

	t.Run("go mod edit error", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/go.mod": "module old\n",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.RunGoModEdit = func(string, string) error {
			return errors.New("go mod fail")
		}

		err := NewProjectWithDeps(targetDir, "", "example.com/mod", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "updating go.mod module") {
			t.Errorf("expected error to contain 'updating go.mod module', got: %v", err)
		}
		if !strings.Contains(err.Error(), "go mod fail") {
			t.Errorf("expected error to contain 'go mod fail', got: %v", err)
		}
	})

	t.Run("copy dir read error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.ReadFile = func(string) ([]byte, error) {
			return nil, errors.New("read fail")
		}

		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "reading file") {
			t.Errorf("expected error to contain 'reading file', got: %v", err)
		}
		if !strings.Contains(err.Error(), "read fail") {
			t.Errorf("expected error to contain 'read fail', got: %v", err)
		}
	})

	t.Run("copy dir write error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.WriteFile = func(string, []byte, os.FileMode) error {
			return errors.New("write fail")
		}

		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "writing file") {
			t.Errorf("expected error to contain 'writing file', got: %v", err)
		}
		if !strings.Contains(err.Error(), "write fail") {
			t.Errorf("expected error to contain 'write fail', got: %v", err)
		}
	})

	t.Run("copy dir mkdir all error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		callCount := 0
		deps.MkdirAll = func(string, os.FileMode) error {
			callCount++
			if callCount > 1 {
				return errors.New("mkdir fail")
			}
			return nil
		}

		err := NewProjectWithDeps(t.TempDir(), "", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "creating destination directory") {
			t.Errorf("expected error to contain 'creating destination directory', got: %v", err)
		}
		if !strings.Contains(err.Error(), "mkdir fail") {
			t.Errorf("expected error to contain 'mkdir fail', got: %v", err)
		}
	})

	t.Run("update sonar props read error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/sonar-project.properties": "sonar.projectKey=old",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		callCount := 0
		deps.ReadFile = func(name string) ([]byte, error) {
			callCount++
			if callCount > 1 {
				return nil, errors.New("read fail")
			}
			return os.ReadFile(name)
		}

		err := NewProjectWithDeps(t.TempDir(), "myapp", "", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "reading sonar properties") {
			t.Errorf("expected error to contain 'reading sonar properties', got: %v", err)
		}
		if !strings.Contains(err.Error(), "read fail") {
			t.Errorf("expected error to contain 'read fail', got: %v", err)
		}
	})

	t.Run("mkdir all error for directory entry in zip", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		body := newFakeZipWithDirs(t, nil, map[string]os.FileMode{
			"go-stack-main/internal/scaffold/template/dir/": 0o750,
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.MkdirAll = func(string, os.FileMode) error {
			return errors.New("mkdir fail")
		}

		_, err := extractTemplate(deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "creating directory") {
			t.Errorf("expected error to contain 'creating directory', got: %v", err)
		}
		if !strings.Contains(err.Error(), "mkdir fail") {
			t.Errorf("expected error to contain 'mkdir fail', got: %v", err)
		}
	})
}

func TestNewProject(t *testing.T) {
	t.Parallel()

	t.Run("uses default dependencies", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := newFakeZip(t, map[string]string{
				"go-stack-main/internal/scaffold/template/foo.txt": "bar",
			})
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(body); err != nil {
				t.Logf("failed to write response: %v", err)
			}
		}))
		defer server.Close()

		origURL := zipURL
		zipURL = server.URL
		defer func() { zipURL = origURL }()

		targetDir := t.TempDir()
		err := NewProject(targetDir, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(targetDir, "foo.txt"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(content) != "bar" {
			t.Errorf("expected 'bar', got: %s", string(content))
		}
	})
}

func Test_copyDir(t *testing.T) {
	t.Parallel()

	t.Run("read dir error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		err := copyDir(deps, "/nonexistent/path", t.TempDir())
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "reading source directory") {
			t.Errorf("expected error to contain 'reading source directory', got: %v", err)
		}
	})

	t.Run("mkdir all error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		src := t.TempDir()
		err := copyDir(deps, src, "/proc/invalid")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "creating destination directory") {
			t.Errorf("expected error to contain 'creating destination directory', got: %v", err)
		}
	})

	t.Run("nested copy succeeds", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		src := t.TempDir()
		dst := t.TempDir()
		if err := os.MkdirAll(filepath.Join(src, "nested"), 0o750); err != nil {
			t.Fatalf("failed to create nested dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(src, "nested", "file.txt"), []byte("hello"), 0o640); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		err := copyDir(deps, src, dst)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(dst, "nested", "file.txt"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(content) != "hello" {
			t.Errorf("expected 'hello', got: %s", string(content))
		}
	})

	t.Run("nested copy error bubbles up", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		src := t.TempDir()
		dst := t.TempDir()
		if err := os.WriteFile(filepath.Join(src, "file1.txt"), []byte("data1"), 0o640); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(src, "sub"), 0o750); err != nil {
			t.Fatalf("failed to create sub dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(src, "sub", "file2.txt"), []byte("data2"), 0o640); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		callCount := 0
		deps.ReadFile = func(name string) ([]byte, error) {
			callCount++
			if callCount > 1 {
				return nil, errors.New("read fail")
			}
			return os.ReadFile(name)
		}

		err := copyDir(deps, src, dst)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "reading file") {
			t.Errorf("expected error to contain 'reading file', got: %v", err)
		}
		if !strings.Contains(err.Error(), "read fail") {
			t.Errorf("expected error to contain 'read fail', got: %v", err)
		}
	})
}

func TestNewProjectWithDepsRemoveAllError(t *testing.T) {
	t.Parallel()

	t.Run("remove all error on defer does not affect return", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/hello.txt": "world",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.RemoveAll = func(string) error {
			return errors.New("remove fail")
		}

		err := NewProjectWithDeps(targetDir, "", "", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(content) != "world" {
			t.Errorf("expected 'world', got: %s", string(content))
		}
	})
}

func TestNewProjectWithDepsNoGoModModule(t *testing.T) {
	t.Parallel()

	t.Run("no go.mod means no module edit", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/foo.txt": "bar",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		called := false
		deps.RunGoModEdit = func(string, string) error {
			called = true
			return nil
		}

		err := NewProjectWithDeps(targetDir, "", "example.com/mod", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Error("expected RunGoModEdit to not be called")
		}
	})
}

func Test_runGoModEdit(t *testing.T) {
	t.Parallel()

	t.Run("missing go.mod returns nil", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		err := runGoModEdit(dir, "example.com/mod")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("existing go.mod is updated", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("module old\n"), 0o640); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}
		err := runGoModEdit(dir, "example.com/mod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, err := os.ReadFile(goModPath)
		if err != nil {
			t.Fatalf("failed to read go.mod: %v", err)
		}
		if !strings.Contains(string(content), "module example.com/mod") {
			t.Errorf("expected module to be updated, got: %s", string(content))
		}
	})
}

func Test_updateSonarProps(t *testing.T) {
	t.Parallel()

	t.Run("read error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		deps.ReadFile = func(string) ([]byte, error) {
			return nil, errors.New("read fail")
		}
		err := updateSonarProps(deps, "/tmp", "myapp")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "reading sonar properties") {
			t.Errorf("expected error to contain 'reading sonar properties', got: %v", err)
		}
		if !strings.Contains(err.Error(), "read fail") {
			t.Errorf("expected error to contain 'read fail', got: %v", err)
		}
	})

	t.Run("write error", func(t *testing.T) {
		t.Parallel()
		deps := DefaultDependencies()
		deps.ReadFile = func(string) ([]byte, error) {
			return []byte("sonar.projectKey=old"), nil
		}
		deps.WriteFile = func(string, []byte, os.FileMode) error {
			return errors.New("write fail")
		}
		err := updateSonarProps(deps, "/tmp", "myapp")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "writing sonar properties") {
			t.Errorf("expected error to contain 'writing sonar properties', got: %v", err)
		}
		if !strings.Contains(err.Error(), "write fail") {
			t.Errorf("expected error to contain 'write fail', got: %v", err)
		}
	})

	t.Run("replaces both keys", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		path := filepath.Join(targetDir, "sonar-project.properties")
		if err := os.WriteFile(path, []byte("sonar.projectKey=oldKey\nsonar.projectName=oldName\n"), 0o640); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		deps := DefaultDependencies()
		err := updateSonarProps(deps, targetDir, "newapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if !strings.Contains(string(content), "sonar.projectKey=newapp") {
			t.Errorf("expected sonar.projectKey=newapp, got: %s", string(content))
		}
		if !strings.Contains(string(content), "sonar.projectName=newapp") {
			t.Errorf("expected sonar.projectName=newapp, got: %s", string(content))
		}
	})

	t.Run("does not replace unrelated lines", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		path := filepath.Join(targetDir, "sonar-project.properties")
		input := "sonar.projectKey=oldKey\nsonar.sources=.\nsonar.projectName=oldName\n"
		if err := os.WriteFile(path, []byte(input), 0o640); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		deps := DefaultDependencies()
		err := updateSonarProps(deps, targetDir, "newapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if !strings.Contains(string(content), "sonar.sources=.") {
			t.Errorf("expected sonar.sources=. to remain unchanged, got: %s", string(content))
		}
	})
}

func Test_defaultDirMode(t *testing.T) {
	t.Parallel()

	t.Run("zero returns default", func(t *testing.T) {
		t.Parallel()
		if defaultDirMode(0) != 0o750 {
			t.Errorf("expected 0750, got: %o", defaultDirMode(0))
		}
	})

	t.Run("non-zero returns input", func(t *testing.T) {
		t.Parallel()
		if defaultDirMode(0o700) != 0o700 {
			t.Errorf("expected 0700, got: %o", defaultDirMode(0o700))
		}
	})
}

func TestStatusCodeError(t *testing.T) {
	t.Parallel()
	e := statusCodeError(404)
	if e.Error() != "unexpected status code: 404" {
		t.Errorf("expected 'unexpected status code: 404', got: %s", e.Error())
	}
}

func TestDefaultDependencies(t *testing.T) {
	t.Parallel()
	deps := DefaultDependencies()
	if deps.HTTPGet == nil {
		t.Error("expected HTTPGet to be set")
	}
	if deps.MkdirAll == nil {
		t.Error("expected MkdirAll to be set")
	}
	if deps.MkdirTemp == nil {
		t.Error("expected MkdirTemp to be set")
	}
	if deps.RemoveAll == nil {
		t.Error("expected RemoveAll to be set")
	}
	if deps.Create == nil {
		t.Error("expected Create to be set")
	}
	if deps.WriteFile == nil {
		t.Error("expected WriteFile to be set")
	}
	if deps.ReadFile == nil {
		t.Error("expected ReadFile to be set")
	}
	if deps.RunGoModEdit == nil {
		t.Error("expected RunGoModEdit to be set")
	}
	if deps.OpenZipFile == nil {
		t.Error("expected OpenZipFile to be set")
	}
	if deps.Chmod == nil {
		t.Error("expected Chmod to be set")
	}
}

func TestReplaceTemplateImports(t *testing.T) {
	t.Parallel()

	t.Run("rewrites occurrences in regular files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "main.go")
		original := []byte("import \"" + templateImportPath + "/sub\"\n")
		if err := os.WriteFile(file, original, 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := replaceTemplateImports(DefaultDependencies(), dir, "example.com/myapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		want := "import \"example.com/myapp/sub\"\n"
		if string(got) != want {
			t.Errorf("expected %q, got %q", want, string(got))
		}
	})

	t.Run("skips files without the template path", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "noop.go")
		if err := os.WriteFile(file, []byte("package main\n"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		deps := DefaultDependencies()
		writeCalled := false
		deps.WriteFile = func(string, []byte, os.FileMode) error {
			writeCalled = true
			return nil
		}

		err := replaceTemplateImports(deps, dir, "example.com/myapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if writeCalled {
			t.Error("expected WriteFile not to be called")
		}
	})

	t.Run("walk error is propagated", func(t *testing.T) {
		t.Parallel()
		err := replaceTemplateImports(DefaultDependencies(), filepath.Join(t.TempDir(), "missing"), "example.com/m")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("read file error is wrapped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("hello"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		deps := DefaultDependencies()
		deps.ReadFile = func(string) ([]byte, error) {
			return nil, errors.New("read fail")
		}

		err := replaceTemplateImports(deps, dir, "example.com/m")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "reading file") {
			t.Errorf("expected 'reading file' in error, got: %v", err)
		}
	})

	t.Run("write file error is wrapped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "main.go")
		if err := os.WriteFile(file, []byte(templateImportPath), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		deps := DefaultDependencies()
		deps.WriteFile = func(string, []byte, os.FileMode) error {
			return errors.New("write fail")
		}

		err := replaceTemplateImports(deps, dir, "example.com/m")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "writing file") {
			t.Errorf("expected 'writing file' in error, got: %v", err)
		}
	})

	t.Run("zero perm falls back to default", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "main.go")
		if err := os.WriteFile(file, []byte(""), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if err := os.Chmod(file, 0); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(file, 0o600) })

		var capturedPerm os.FileMode
		deps := DefaultDependencies()
		deps.ReadFile = func(string) ([]byte, error) {
			return []byte(templateImportPath), nil
		}
		deps.WriteFile = func(_ string, _ []byte, perm os.FileMode) error {
			capturedPerm = perm
			return nil
		}

		err := replaceTemplateImports(deps, dir, "example.com/m")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedPerm != 0o640 {
			t.Errorf("expected perm 0o640, got %o", capturedPerm)
		}
	})
}

func TestNewProjectWithDepsReplaceTemplateImports(t *testing.T) {
	t.Parallel()

	t.Run("rewrites template imports after go mod edit", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/go.mod":      "module old\n",
			"go-stack-main/internal/scaffold/template/app/main.go": "import \"" + templateImportPath + "/foo\"\n",
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.RunGoModEdit = func(string, string) error { return nil }

		err := NewProjectWithDeps(targetDir, "", "example.com/myapp", deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(targetDir, "app", "main.go"))
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		want := "import \"example.com/myapp/foo\"\n"
		if string(got) != want {
			t.Errorf("expected %q, got %q", want, string(got))
		}
	})

	t.Run("replace error is wrapped", func(t *testing.T) {
		t.Parallel()
		targetDir := t.TempDir()
		deps := DefaultDependencies()
		body := newFakeZip(t, map[string]string{
			"go-stack-main/internal/scaffold/template/go.mod":      "module old\n",
			"go-stack-main/internal/scaffold/template/app/main.go": templateImportPath,
		})
		deps.HTTPGet = func(string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}
		deps.RunGoModEdit = func(string, string) error { return nil }
		realWrite := deps.WriteFile
		deps.WriteFile = func(name string, data []byte, perm os.FileMode) error {
			if bytes.Contains(data, []byte("example.com/myapp")) {
				return errors.New("write fail")
			}
			return realWrite(name, data, perm)
		}

		err := NewProjectWithDeps(targetDir, "", "example.com/myapp", deps)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "replacing template imports") {
			t.Errorf("expected 'replacing template imports' in error, got: %v", err)
		}
	})
}

// BenchmarkNewProject benchmarks the project creation process.
func BenchmarkNewProject(b *testing.B) {
	body := newFakeZip(b, map[string]string{
		"go-stack-main/internal/scaffold/template/hello.txt": "world",
	})
	deps := DefaultDependencies()
	deps.HTTPGet = func(string) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	}

	b.ResetTimer()
	for i := range b.N {
		targetDir := filepath.Join(b.TempDir(), fmt.Sprintf("proj%d", i))
		_ = NewProjectWithDeps(targetDir, "app", "example.com/mod", deps)
	}
}
