// Package httpsvr implements the core HTTP server functionality and shared components
package httpsvr

import (
	"context"
	"net/http"
)

type HTTPMethod string

const (
	MethodGet     HTTPMethod = http.MethodGet
	MethodPost    HTTPMethod = http.MethodPost
	MethodPut     HTTPMethod = http.MethodPut
	MethodDelete  HTTPMethod = http.MethodDelete
	MethodPatch   HTTPMethod = http.MethodPatch
	MethodHead    HTTPMethod = http.MethodHead
	MethodOptions HTTPMethod = http.MethodOptions
	MethodTrace   HTTPMethod = http.MethodTrace
	MethodConnect HTTPMethod = http.MethodConnect
)

type HTTPServer interface {
	// Start starts the HTTP server
	Start(ctx context.Context) error

	// Stop gracefully shuts down the server
	Stop(ctx context.Context) error
}

var _ HTTPServer = (*defaultHTTPServer)(nil)
