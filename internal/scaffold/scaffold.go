// Package scaffold provides functionality to create new Go projects from a remote
// template repository.
package scaffold

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//nolint:gochecknoglobals
var zipURL = "https://github.com/emilioforrer/go-stack/archive/refs/heads/main.zip"

const (
	zipPrefix          = "go-stack-main/internal/scaffold/template/"
	sonarKey           = "sonar.projectKey"
	sonarName          = "sonar.projectName"
	templateImportPath = "github.com/emilioforrer/go-stack/internal/scaffold/template"
)

// ErrTargetPathRequired is returned when the target path argument is empty.
var ErrTargetPathRequired = errors.New("target path is required")

// Dependencies holds external dependencies that can be swapped in tests.
type Dependencies struct {
	HTTPGet      func(url string) (*http.Response, error)
	MkdirAll     func(path string, perm os.FileMode) error
	MkdirTemp    func(dir, pattern string) (string, error)
	RemoveAll    func(path string) error
	Create       func(name string) (*os.File, error)
	WriteFile    func(name string, data []byte, perm os.FileMode) error
	ReadFile     func(name string) ([]byte, error)
	RunGoModEdit func(targetDir, module string) error
	OpenZipFile  func(f *zip.File) (io.ReadCloser, error)
	Chmod        func(name string, mode os.FileMode) error
}

// DefaultDependencies returns the production implementations.
func DefaultDependencies() Dependencies {
	return Dependencies{
		HTTPGet:      http.Get,
		MkdirAll:     os.MkdirAll,
		MkdirTemp:    os.MkdirTemp,
		RemoveAll:    os.RemoveAll,
		Create:       os.Create,
		WriteFile:    os.WriteFile,
		ReadFile:     os.ReadFile,
		RunGoModEdit: runGoModEdit,
		OpenZipFile:  func(f *zip.File) (io.ReadCloser, error) { return f.Open() },
		Chmod:        os.Chmod,
	}
}

func runGoModEdit(targetDir, module string) error {
	goModPath := filepath.Join(targetDir, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return nil
	}
	//nolint:gosec,noctx // G204 intentional CLI feature; CommandContext not needed here.
	cmd := exec.Command("go", "mod", "edit", "-module="+module)
	cmd.Dir = targetDir
	return cmd.Run()
}

// NewProject creates a new project at targetPath from the remote template.
func NewProject(targetPath, name, module string) error {
	return NewProjectWithDeps(targetPath, name, module, DefaultDependencies())
}

// NewProjectWithDeps creates a new project with injectable dependencies.
func NewProjectWithDeps(targetPath, name, module string, deps Dependencies) error {
	if targetPath == "" {
		return ErrTargetPathRequired
	}

	tmpDir, err := extractTemplate(deps)
	if err != nil {
		return fmt.Errorf("extracting template: %w", err)
	}
	defer func() {
		_ = deps.RemoveAll(tmpDir)
	}()

	err = copyDir(deps, tmpDir, targetPath)
	if err != nil {
		return fmt.Errorf("copying template: %w", err)
	}

	if name != "" {
		err = updateSonarProps(deps, targetPath, name)
		if err != nil {
			return fmt.Errorf("updating sonar properties: %w", err)
		}
	}

	if module != "" {
		goModPath := filepath.Join(targetPath, "go.mod")
		if _, serr := os.Stat(goModPath); serr == nil {
			err = deps.RunGoModEdit(targetPath, module)
			if err != nil {
				return fmt.Errorf("updating go.mod module: %w", err)
			}
			err = replaceTemplateImports(deps, targetPath, module)
			if err != nil {
				return fmt.Errorf("replacing template imports: %w", err)
			}
		}
	}

	return nil
}

func replaceTemplateImports(deps Dependencies, targetPath, module string) error {
	old := []byte(templateImportPath)
	replacement := []byte(module)

	return filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		data, readErr := deps.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading file: %w", readErr)
		}
		if !bytes.Contains(data, old) {
			return nil
		}
		perm := info.Mode().Perm()
		if perm == 0 {
			perm = 0o640
		}
		writeErr := deps.WriteFile(path, bytes.ReplaceAll(data, old, replacement), perm)
		if writeErr != nil {
			return fmt.Errorf("writing file: %w", writeErr)
		}
		return nil
	})
}

func extractTemplate(deps Dependencies) (string, error) {
	resp, err := deps.HTTPGet(zipURL)
	if err != nil {
		return "", fmt.Errorf("downloading zip: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %w", statusCodeError(resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading zip body: %w", err)
	}

	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}

	tmpDir, err := deps.MkdirTemp("", "go-stack-template-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, zipPrefix) || f.Name == zipPrefix {
			continue
		}

		rel := strings.TrimPrefix(f.Name, zipPrefix)
		target := filepath.Join(tmpDir, rel)
		if f.FileInfo().IsDir() {
			err = deps.MkdirAll(target, defaultDirMode(f.Mode()))
			if err != nil {
				cleanup(deps, tmpDir)
				return "", fmt.Errorf("creating directory: %w", err)
			}
			continue
		}

		err = deps.MkdirAll(filepath.Dir(target), 0o750)
		if err != nil {
			cleanup(deps, tmpDir)
			return "", fmt.Errorf("creating parent directory: %w", err)
		}

		err = extractFile(deps, f, target)
		if err != nil {
			cleanup(deps, tmpDir)
			return "", err
		}
	}

	return tmpDir, nil
}

func defaultDirMode(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o750
	}
	return mode
}

func extractFile(deps Dependencies, f *zip.File, target string) error {
	rc, err := deps.OpenZipFile(f)
	if err != nil {
		return fmt.Errorf("opening zip file: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	out, err := deps.Create(target)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer func() {
		_ = out.Close()
	}()

	_, copyErr := io.Copy(out, rc)
	if copyErr != nil {
		return fmt.Errorf("writing file: %w", copyErr)
	}

	if f.Mode() != 0 {
		err = deps.Chmod(target, f.Mode())
		if err != nil {
			return fmt.Errorf("setting file permissions: %w", err)
		}
	}

	return nil
}

func cleanup(deps Dependencies, path string) {
	_ = deps.RemoveAll(path)
}

func copyDir(deps Dependencies, src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading source directory: %w", err)
	}

	if err := deps.MkdirAll(dst, 0o750); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			copyErr := copyDir(deps, srcPath, dstPath)
			if copyErr != nil {
				return copyErr
			}
			continue
		}

		data, readErr := deps.ReadFile(srcPath)
		if readErr != nil {
			return fmt.Errorf("reading file: %w", readErr)
		}
		perm := entry.Type().Perm()
		if perm == 0 {
			perm = 0o640
		}
		if writeErr := deps.WriteFile(dstPath, data, perm); writeErr != nil {
			return fmt.Errorf("writing file: %w", writeErr)
		}
	}

	return nil
}

func updateSonarProps(deps Dependencies, targetPath, name string) error {
	path := filepath.Join(targetPath, "sonar-project.properties")
	data, err := deps.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading sonar properties: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, sonarKey+"=") {
			lines[i] = sonarKey + "=" + name
		} else if strings.HasPrefix(line, sonarName+"=") {
			lines[i] = sonarName + "=" + name
		}
	}

	err = deps.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o640)
	if err != nil {
		return fmt.Errorf("writing sonar properties: %w", err)
	}

	return nil
}

type statusCodeError int

func (e statusCodeError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", int(e))
}
