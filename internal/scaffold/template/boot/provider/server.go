package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/emilioforrer/go-stack/pkg/httpsvr"
	"github.com/samber/do/v2"
)

type ServerOptions struct {
	Debug bool
	Host  string
	Port  int
}

type ServerProvider struct {
	options ServerOptions
	server  httpsvr.HTTPServer
	addr    string
}

var (
	_ boot.Provider = (*ServerProvider)(nil)

	ErrServerNotInitialized = errors.New("server provider: server not initialized, call Register first")

	defaultHost = "0.0.0.0"
	defaultPort = 8888
)

func (p *ServerProvider) Register(_ context.Context, container boot.Container) error {
	router := httpsvr.NewDefaultRouter()
	router.Use(httpsvr.RequestLoggerMiddleware(p.options.Debug))
	api := humago.New(router, huma.DefaultConfig("My API", "1.0.0"))

	// Register routes and API with the container for later use
	do.ProvideValue(container.Injector(), router)
	do.ProvideValue(container.Injector(), api)

	addr := fmt.Sprintf("%s:%d", p.options.Host, p.options.Port)
	config := httpsvr.NewDefaultConfig(router)
	config.Addr = addr
	config.ReadHeaderTimeout = 5 * time.Second

	p.setServer(httpsvr.NewDefaultHTTPServer(config), addr)

	return nil
}

func (p *ServerProvider) setServer(server httpsvr.HTTPServer, addr string) {
	p.server = server
	p.addr = addr
}

func (p *ServerProvider) Boot(ctx context.Context, _ boot.Container) error {
	if p.server == nil {
		return ErrServerNotInitialized
	}

	go func() {
		slog.Info("Server starting", "addr", p.addr)
		if err := p.server.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
		}
	}()

	return nil
}

func (p *ServerProvider) Shutdown(_ context.Context, _ boot.Container) error {
	if p.server == nil {
		return nil
	}

	slog.Info("Server shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.server.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	return nil
}

func NewServerProvider(opts ...ServerOptions) *ServerProvider {
	o := ServerOptions{
		Host: defaultHost,
		Port: defaultPort,
	}
	if len(opts) > 0 {
		o = opts[0]
		if o.Host == "" {
			o.Host = defaultHost
		}
		if o.Port == 0 {
			o.Port = defaultPort
		}
	}
	return &ServerProvider{options: o}
}
