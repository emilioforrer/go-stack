# httpsvr

An HTTP server package built on Go's `net/http`, providing a structured router, configurable server, middleware chaining, and JSON error responses.

## Install

```bash
go get github.com/emilioforrer/go-stack/pkg/httpsvr
```

## Key Concepts

| Type | Role |
|---|---|
| `HTTPServer` | Interface with `Start`/`Stop` lifecycle methods |
| `DefaultHTTPServer` | Standard `net/http.Server` implementation of `HTTPServer` |
| `DefaultConfig` | Server configuration (address, timeouts, handler) |
| `Router` | Interface for method-based route registration with per-route middleware |
| `DefaultRouter` | `http.ServeMux`-backed implementation of `Router` |
| `App` | Interface for running and stopping the server with signal handling |
| `DefaultApp` | Full lifecycle app with graceful shutdown and OS signal handling |
| `Middleware` | `func(http.Handler) http.Handler` — composable middleware type |

## Built-in Middleware

| Middleware | Description |
|---|---|
| `RequestLoggerMiddleware` | Rails-style request logging with status code and duration |
| `NotFoundMiddleware` | Returns JSON 404 responses for unmatched routes |
| `InternalServerErrorMiddleware` | Returns JSON 5xx responses |
| `ClientErrorMiddleware` | Returns JSON 4xx responses (excluding 404) |
| `TracingMiddleware` | Attaches a trace ID to the request context |
| `RecoveryMiddleware` | Recovers from panics and returns 500 |

## Usage

### Basic Server with App

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/emilioforrer/go-stack/pkg/httpsvr"
)

func main() {
    app := httpsvr.NewApp()

    app.Router.GET("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })

    app.Router.GET("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "OK")
    })

    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

### Custom Configuration

```go
router := httpsvr.NewDefaultRouter()
router.Use(httpsvr.TracingMiddleware())
router.Use(httpsvr.RecoveryMiddleware())
router.Use(httpsvr.NotFoundMiddleware())
router.Use(httpsvr.InternalServerErrorMiddleware())
router.Use(httpsvr.ClientErrorMiddleware())

config := httpsvr.NewDefaultConfig(router)
config.Addr = ":8080"
config.ReadTimeout = 10 * time.Second
config.WriteTimeout = 10 * time.Second

server := httpsvr.NewDefaultHTTPServer(config)
```

### Per-Route Middleware

```go
router := httpsvr.NewDefaultRouter()

authMiddleware := func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}

router.GET("/public", publicHandler)
router.GET("/private", privateHandler, authMiddleware)
```

### JSON Error Format

All error middlewares return responses in JSON API style:

```json
{
  "errors": [
    {
      "id": "0af7651916cd43dd8448af2119c07993",
      "status": "404",
      "title": "Route not found",
      "detail": "The requested route was not found",
      "source": {
        "pointer": "/api/unknown"
      }
    }
  ]
}
```
