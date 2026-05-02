package httpsvr

import (
	"fmt"
	"net/http"
	"strings"
)

type Mux interface {
	Handler() http.Handler
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
	ServeHTTP(w http.ResponseWriter, req *http.Request)
}

type Router interface {
	Mux
	GET(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	POST(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	PUT(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	DELETE(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	PATCH(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	HEAD(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	OPTIONS(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	TRACE(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	CONNECT(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware)
	RegisterRoutes(routes Routes)
	Use(middleware Middleware)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
	ServeHTTP(w http.ResponseWriter, req *http.Request)
}

type Routes map[string]http.HandlerFunc

type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

type Middleware func(http.Handler) http.Handler

// DefaultRouter encapsulates http.ServeMux.
type DefaultRouter struct {
	mux         *http.ServeMux
	middlewares []Middleware
}

var _ http.Handler = (*DefaultRouter)(nil)

// NewDefaultRouter initializes and returns a new DefaultRouter
func NewDefaultRouter() *DefaultRouter {
	return &DefaultRouter{
		mux:         http.NewServeMux(),
		middlewares: []Middleware{},
	}
}

// Use adds a middleware to the router
func (r *DefaultRouter) Use(middleware Middleware) {
	r.middlewares = append(r.middlewares, middleware)
}

// Chain creates a new http.HandlerFunc by chaining multiple middlewares
func (r *DefaultRouter) Chain(middlewares ...Middleware) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Create handler chain
			handler := http.Handler(next)

			// Apply middlewares in reverse order
			for i := len(middlewares) - 1; i >= 0; i-- {
				handler = middlewares[i](handler)
			}

			handler.ServeHTTP(w, r)
		}
	}
}

// GET registers a new GET route
func (r *DefaultRouter) GET(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodGet, path, handlerFunc, middlewares...)
}

// POST registers a new POST route
func (r *DefaultRouter) POST(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodPost, path, handlerFunc, middlewares...)
}

// PUT registers a new PUT route
func (r *DefaultRouter) PUT(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodPut, path, handlerFunc, middlewares...)
}

// DELETE registers a new DELETE route
func (r *DefaultRouter) DELETE(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodDelete, path, handlerFunc, middlewares...)
}

// PATCH registers a new PATCH route
func (r *DefaultRouter) PATCH(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodPatch, path, handlerFunc, middlewares...)
}

// HEAD registers a new HEAD route
func (r *DefaultRouter) HEAD(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodHead, path, handlerFunc, middlewares...)
}

// OPTIONS registers a new OPTIONS route
func (r *DefaultRouter) OPTIONS(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodOptions, path, handlerFunc, middlewares...)
}

// TRACE registers a new TRACE route
func (r *DefaultRouter) TRACE(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodTrace, path, handlerFunc, middlewares...)
}

// CONNECT registers a new CONNECT route
func (r *DefaultRouter) CONNECT(path string, handlerFunc http.HandlerFunc, middlewares ...Middleware) {
	r.RegisterRoute(MethodConnect, path, handlerFunc, middlewares...)
}

// RegisterRoute allows adding routes to the router with optional middlewares
func (r *DefaultRouter) RegisterRoute(
	method HTTPMethod,
	path string,
	handlerFunc http.HandlerFunc,
	middlewares ...Middleware,
) {
	// Wrap handler with middlewares
	finalHandler := handlerFunc
	if len(middlewares) > 0 {
		finalHandler = r.Chain(middlewares...)(handlerFunc)
	}

	// Go 1.23+ requires explicit method-based route registration
	r.mux.Handle(fmt.Sprintf("%s %s", method, path), finalHandler)

	// if len(middlewares) > 0 {
	// 	// Apply route-specific middlewares using Chain
	// 	chainedHandler := r.Chain(middlewares...)(handlerFunc)
	// 	r.mux.HandleFunc(fmt.Sprintf("%s %s", method, path), chainedHandler)
	// 	return
	// }
	// r.mux.HandleFunc(path, handlerFunc)
}

func (r *DefaultRouter) RegisterRoutes(routes Routes) {
	for key, handlerFunc := range routes {
		parts := strings.Split(key, " ")
		method, path := HTTPMethod(parts[0]), parts[1]
		r.RegisterRoute(method, path, handlerFunc)
	}
}

// HandleFunc registers a handler function for the given pattern.
func (r *DefaultRouter) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.mux.HandleFunc(pattern, handler)
}

func (r *DefaultRouter) Handler() http.Handler {
	return r.mux
}

func (r *DefaultRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Create a chain of handlers using the middlewares
	var handler http.Handler = r.mux

	// Apply middlewares in reverse order
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}

	// Serve the request with all middlewares applied
	handler.ServeHTTP(w, req)
}
