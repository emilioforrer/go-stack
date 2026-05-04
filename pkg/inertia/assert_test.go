package inertia

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

// assertEqual fails the test if got != want.
func assertEqual(t *testing.T, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// assertTrue fails the test if v is false.
func assertTrue(t *testing.T, v bool) {
	t.Helper()
	if !v {
		t.Errorf("expected true, got false")
	}
}

// assertFalse fails the test if v is true.
func assertFalse(t *testing.T, v bool) {
	t.Helper()
	if v {
		t.Errorf("expected false, got true")
	}
}

// isNil reports whether v is nil, handling typed nil pointers in interfaces.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

// assertNil fails the test if v is not nil.
func assertNil(t *testing.T, v any) {
	t.Helper()
	if !isNil(v) {
		t.Errorf("expected nil, got %+v", v)
	}
}

// assertNotNil fails the test if v is nil.
func assertNotNil(t *testing.T, v any) {
	t.Helper()
	if v == nil {
		t.Errorf("expected non-nil, got nil")
	}
}

// assertContains fails the test if s does not contain substr.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("%q does not contain %q", s, substr)
	}
}

// assertErrorIs fails the test if err does not wrap target.
func assertErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("error %v does not wrap %v", err, target)
	}
}

// assertLen fails the test if the length of v is not n.
func assertLen(t *testing.T, v any, n int) {
	t.Helper()
	if reflect.ValueOf(v).Len() != n {
		t.Errorf("expected length %d, got %d", n, reflect.ValueOf(v).Len())
	}
}

// requireEqual fatals the test if got != want.
func requireEqual(t *testing.T, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// requireNoError fatals the test if err is not nil.
func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// requireError fatals the test if err is nil.
func requireError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// requireNil fatals the test if v is not nil.
func requireNil(t *testing.T, v any) {
	t.Helper()
	if !isNil(v) {
		t.Fatalf("expected nil, got %+v", v)
	}
}

// requireNotNil fatals the test if v is nil.
func requireNotNil(t *testing.T, v any) {
	t.Helper()
	if v == nil {
		t.Fatalf("expected non-nil, got nil")
	}
}

// requireContains fatals the test if s does not contain substr.
func requireContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("%q does not contain %q", s, substr)
	}
}

// requireErrorIs fatals the test if err does not wrap target.
func requireErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error %v does not wrap %v", err, target)
	}
}

// requireContainsError fatals the test if err.Error() does not contain substr.
func requireContainsError(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("error %q does not contain %q", err.Error(), substr)
	}
}

// requireWriteFile is a helper that writes data to path and fatals on error.
func requireWriteFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

// requireMkdirAll is a helper that creates directories and fatals on error.
func requireMkdirAll(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, perm); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// requireChdir is a helper that changes directory and fatals on error.
func requireChdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

// assertContainsError fails the test if err.Error() does not contain substr.
func assertContainsError(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected error containing %q, got nil", substr)
		return
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("error %q does not contain %q", err.Error(), substr)
	}
}
