package boot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/samber/do/v2"
)

var ErrBootBeforeRegister = errors.New("bootstrapper: Boot called before Register")

type Container interface {
	Injector() do.Injector
}

type Provider interface {
	Register(ctx context.Context, container Container) error
	Boot(ctx context.Context, container Container) error
	Shutdown(ctx context.Context, container Container) error
}

type Registrar interface {
	Register(ctx context.Context) error
}

type Lifecycle interface {
	Boot(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type HasContainer interface {
	Container() Container
}

type ProviderRegistry interface {
	Providers() []Provider
}

type Bootstrapper interface {
	HasContainer
	ProviderRegistry
	Registrar
	Lifecycle
}

var _ Container = (*DefaultContainer)(nil)

type DefaultContainer struct {
	injector do.Injector
}

func NewContainer(i do.Injector) *DefaultContainer {
	return &DefaultContainer{
		injector: i,
	}
}

func (c *DefaultContainer) Injector() do.Injector {
	return c.injector
}

var _ Provider = (*DefaultProvider)(nil)

type DefaultProvider struct{}

func (s *DefaultProvider) Register(ctx context.Context, container Container) error {
	return nil
}

func (s *DefaultProvider) Boot(ctx context.Context, container Container) error {
	return nil
}

func (s *DefaultProvider) Shutdown(ctx context.Context, container Container) error {
	return nil
}

var _ Bootstrapper = (*DefaultBootstrapper)(nil)

type DefaultBootstrapper struct {
	container    *DefaultContainer
	providers    []Provider
	booted       bool
	serviceMutex sync.Mutex
	registered   bool
	shutDown     bool
}

func NewDefaultBootstrapper(i do.Injector, providers ...Provider) *DefaultBootstrapper {
	return &DefaultBootstrapper{
		container:    NewContainer(i),
		providers:    providers,
		booted:       false,
		registered:   false,
		serviceMutex: sync.Mutex{},
	}
}

func (b *DefaultBootstrapper) Container() Container {
	return b.container
}

func (b *DefaultBootstrapper) Register(ctx context.Context) error {
	if b.registered {
		slog.Warn("Bootstrapper already registered")
		return nil
	}

	for _, p := range b.providers {
		if err := p.Register(ctx, b.container); err != nil {
			return fmt.Errorf("%T register: %w", p, err)
		}
	}

	b.registered = true
	return nil
}

func (b *DefaultBootstrapper) Boot(ctx context.Context) error {
	if !b.registered {
		return ErrBootBeforeRegister
	}
	if b.booted {
		slog.Warn("bootstrapper already booted")
		return nil
	}

	for _, p := range b.providers {
		if err := p.Boot(ctx, b.container); err != nil {
			return fmt.Errorf("%T boot: %w", p, err)
		}
	}

	b.booted = true
	return nil
}

func (b *DefaultBootstrapper) Shutdown(ctx context.Context) error {
	if b.shutDown {
		slog.Warn("bootstrapper already shut down")
		return nil
	}

	errs := make([]error, 0, len(b.providers))
	for i := len(b.providers) - 1; i >= 0; i-- {
		p := b.providers[i]
		if err := p.Shutdown(ctx, b.container); err != nil {
			errs = append(errs, fmt.Errorf("%T shutdown: %w", p, err))
		}
	}

	b.shutDown = true

	if len(errs) > 0 {
		slog.Error("shutdown failed", slog.Any("errors", errs))
		return errors.Join(errs...)
	}

	return nil
}

func (b *DefaultBootstrapper) Providers() []Provider {
	b.serviceMutex.Lock()
	defer b.serviceMutex.Unlock()

	out := make([]Provider, len(b.providers))
	copy(out, b.providers)

	return out
}
