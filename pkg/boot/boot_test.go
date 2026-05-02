package boot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/samber/do/v2"
)

// testProvider is a configurable Provider for testing.
type testProvider struct {
	DefaultProvider
	registerFunc func(ctx context.Context, c Container) error
	bootFunc     func(ctx context.Context, c Container) error
	shutdownFunc func(ctx context.Context, c Container) error
}

func (m *testProvider) Register(ctx context.Context, c Container) error {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, c)
	}
	return nil
}

func (m *testProvider) Boot(ctx context.Context, c Container) error {
	if m.bootFunc != nil {
		return m.bootFunc(ctx, c)
	}
	return nil
}

func (m *testProvider) Shutdown(ctx context.Context, c Container) error {
	if m.shutdownFunc != nil {
		return m.shutdownFunc(ctx, c)
	}
	return nil
}

func newInjector() do.Injector {
	return do.New()
}

// --- DefaultContainer tests ---

func TestNewContainer(t *testing.T) {
	i := newInjector()
	c := NewContainer(i)

	if c == nil {
		t.Fatal("expected non-nil container")
	}
	if c.Injector() != i {
		t.Fatal("expected container to return the same injector")
	}
}

// --- DefaultProvider tests ---

func TestDefaultProvider_AllMethodsReturnNil(t *testing.T) {
	p := &DefaultProvider{}
	ctx := context.Background()
	c := NewContainer(newInjector())

	if err := p.Register(ctx, c); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	if err := p.Boot(ctx, c); err != nil {
		t.Fatalf("Boot: unexpected error: %v", err)
	}
	if err := p.Shutdown(ctx, c); err != nil {
		t.Fatalf("Shutdown: unexpected error: %v", err)
	}
}

// --- DefaultBootstrapper tests ---

func TestNewDefaultBootstrapper(t *testing.T) {
	p1 := &testProvider{}
	p2 := &testProvider{}
	b := NewDefaultBootstrapper(newInjector(), p1, p2)

	if b == nil {
		t.Fatal("expected non-nil bootstrapper")
	}
	if b.Container() == nil {
		t.Fatal("expected non-nil container")
	}
	if got := len(b.Providers()); got != 2 {
		t.Fatalf("expected 2 providers, got %d", got)
	}
}

func TestBootstrapper_RegisterAndBoot(t *testing.T) {
	ctx := context.Background()
	var order []string

	p1 := &testProvider{
		registerFunc: func(_ context.Context, _ Container) error { order = append(order, "r1"); return nil },
		bootFunc:     func(_ context.Context, _ Container) error { order = append(order, "b1"); return nil },
	}
	p2 := &testProvider{
		registerFunc: func(_ context.Context, _ Container) error { order = append(order, "r2"); return nil },
		bootFunc:     func(_ context.Context, _ Container) error { order = append(order, "b2"); return nil },
	}

	b := NewDefaultBootstrapper(newInjector(), p1, p2)

	if err := b.Register(ctx); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	if err := b.Boot(ctx); err != nil {
		t.Fatalf("Boot: unexpected error: %v", err)
	}

	expected := []string{"r1", "r2", "b1", "b2"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, order)
		}
	}
}

func TestBootstrapper_Register_Idempotent(t *testing.T) {
	ctx := context.Background()
	calls := 0
	p := &testProvider{registerFunc: func(_ context.Context, _ Container) error { calls++; return nil }}
	b := NewDefaultBootstrapper(newInjector(), p)

	if err := b.Register(ctx); err != nil {
		t.Fatalf("first Register: unexpected error: %v", err)
	}
	if err := b.Register(ctx); err != nil { // second call is a no-op
		t.Fatalf("second Register: unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestBootstrapper_Register_Error(t *testing.T) {
	ctx := context.Background()
	p := &testProvider{registerFunc: func(_ context.Context, _ Container) error { return errors.New("reg fail") }}
	b := NewDefaultBootstrapper(newInjector(), p)

	err := b.Register(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "register") {
		t.Fatalf("expected error to contain 'register', got: %v", err)
	}
	if !strings.Contains(err.Error(), "reg fail") {
		t.Fatalf("expected error to contain 'reg fail', got: %v", err)
	}
}

func TestBootstrapper_Boot_BeforeRegister(t *testing.T) {
	ctx := context.Background()
	b := NewDefaultBootstrapper(newInjector())

	err := b.Boot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Boot called before Register") {
		t.Fatalf("expected error to contain 'Boot called before Register', got: %v", err)
	}
}

func TestBootstrapper_Boot_Idempotent(t *testing.T) {
	ctx := context.Background()
	calls := 0
	p := &testProvider{bootFunc: func(_ context.Context, _ Container) error { calls++; return nil }}
	b := NewDefaultBootstrapper(newInjector(), p)

	if err := b.Register(ctx); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	if err := b.Boot(ctx); err != nil {
		t.Fatalf("first Boot: unexpected error: %v", err)
	}
	if err := b.Boot(ctx); err != nil { // second call is a no-op
		t.Fatalf("second Boot: unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestBootstrapper_Boot_Error(t *testing.T) {
	ctx := context.Background()
	p := &testProvider{bootFunc: func(_ context.Context, _ Container) error { return errors.New("boot fail") }}
	b := NewDefaultBootstrapper(newInjector(), p)

	if err := b.Register(ctx); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	err := b.Boot(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boot") {
		t.Fatalf("expected error to contain 'boot', got: %v", err)
	}
	if !strings.Contains(err.Error(), "boot fail") {
		t.Fatalf("expected error to contain 'boot fail', got: %v", err)
	}
}

func TestBootstrapper_Shutdown_ReverseOrder(t *testing.T) {
	ctx := context.Background()
	var order []string

	p1 := &testProvider{shutdownFunc: func(_ context.Context, _ Container) error { order = append(order, "s1"); return nil }}
	p2 := &testProvider{shutdownFunc: func(_ context.Context, _ Container) error { order = append(order, "s2"); return nil }}
	p3 := &testProvider{shutdownFunc: func(_ context.Context, _ Container) error { order = append(order, "s3"); return nil }}

	b := NewDefaultBootstrapper(newInjector(), p1, p2, p3)

	if err := b.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: unexpected error: %v", err)
	}

	expected := []string{"s3", "s2", "s1"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, order)
		}
	}
}

func TestBootstrapper_Shutdown_Idempotent(t *testing.T) {
	ctx := context.Background()
	calls := 0
	p := &testProvider{shutdownFunc: func(_ context.Context, _ Container) error { calls++; return nil }}
	b := NewDefaultBootstrapper(newInjector(), p)

	if err := b.Shutdown(ctx); err != nil {
		t.Fatalf("first Shutdown: unexpected error: %v", err)
	}
	if err := b.Shutdown(ctx); err != nil { // second call is a no-op
		t.Fatalf("second Shutdown: unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestBootstrapper_Shutdown_CollectsAllErrors(t *testing.T) {
	ctx := context.Background()

	p1 := &testProvider{shutdownFunc: func(_ context.Context, _ Container) error { return errors.New("err1") }}
	p2 := &testProvider{shutdownFunc: func(_ context.Context, _ Container) error { return errors.New("err2") }}

	b := NewDefaultBootstrapper(newInjector(), p1, p2)

	err := b.Shutdown(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	// Both errors should be joined
	if !strings.Contains(err.Error(), "err1") {
		t.Fatalf("expected error to contain 'err1', got: %v", err)
	}
	if !strings.Contains(err.Error(), "err2") {
		t.Fatalf("expected error to contain 'err2', got: %v", err)
	}
}

func TestBootstrapper_Providers_ReturnsCopy(t *testing.T) {
	p1 := &testProvider{}
	p2 := &testProvider{}
	b := NewDefaultBootstrapper(newInjector(), p1, p2)

	providers := b.Providers()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	// Mutating the returned slice should not affect the bootstrapper
	providers[0] = nil
	if b.Providers()[0] == nil {
		t.Fatal("expected Providers to return a defensive copy")
	}
}

func TestBootstrapper_NoProviders(t *testing.T) {
	ctx := context.Background()
	b := NewDefaultBootstrapper(newInjector())

	if err := b.Register(ctx); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	if err := b.Boot(ctx); err != nil {
		t.Fatalf("Boot: unexpected error: %v", err)
	}
	if err := b.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: unexpected error: %v", err)
	}
	if len(b.Providers()) != 0 {
		t.Fatal("expected 0 providers")
	}
}

func TestBootstrapper_ImplementsInterface(t *testing.T) {
	b := NewDefaultBootstrapper(newInjector())
	var _ Bootstrapper = b
}
