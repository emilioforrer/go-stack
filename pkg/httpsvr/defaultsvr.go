package httpsvr

import (
	"context"
	"net/http"
	"time"
)

// DefaultConfig is the default server configuration.
type DefaultConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	Handler           http.Handler
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
}

func NewDefaultConfig(h http.Handler) DefaultConfig {
	return DefaultConfig{
		Addr:              ":4000",
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 30 * time.Second,
		MaxHeaderBytes:    1 << 20,
		Handler:           h,
		ShutdownTimeout:   30 * time.Second,
	}
}

// Example implementation of the Server interface
type defaultHTTPServer struct {
	config DefaultConfig
	server *http.Server
}

func NewDefaultHTTPServer(c DefaultConfig) HTTPServer {
	s := &defaultHTTPServer{
		config: c,
		server: &http.Server{
			Addr:              c.Addr,
			Handler:           c.Handler,
			ReadTimeout:       c.ReadTimeout,
			WriteTimeout:      c.WriteTimeout,
			MaxHeaderBytes:    c.MaxHeaderBytes,
			IdleTimeout:       c.IdleTimeout,
			ReadHeaderTimeout: c.ReadHeaderTimeout,
		},
	}
	return s
}
func (s *defaultHTTPServer) Start(ctx context.Context) error {
	return s.server.ListenAndServe()
}

func (s *defaultHTTPServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
