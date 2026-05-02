package httpsvr

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewDefaultRouter(t *testing.T) {
	r := NewDefaultRouter()
	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if r.mux == nil {
		t.Fatal("expected non-nil mux")
	}
	if len(r.middlewares) != 0 {
		t.Fatalf("expected empty middlewares, got %d", len(r.middlewares))
	}
}

func TestDefaultRouter_Use(t *testing.T) {
	r := NewDefaultRouter()
	called := false
	r.Use(func(next http.Handler) http.Handler {
		called = true
		return next
	})
	if len(r.middlewares) != 1 {
		t.Fatalf("expected 1 middleware, got %d", len(r.middlewares))
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	r.middlewares[0](handler)
	if !called {
		t.Fatal("expected middleware to be called")
	}
}

func TestDefaultRouter_Chain(t *testing.T) {
	r := NewDefaultRouter()
	var order []string
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, req)
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "mw2")
			next.ServeHTTP(w, req)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chained := r.Chain(mw1, mw2)(handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	chained.ServeHTTP(w, req)

	if len(order) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(order))
	}
	if order[0] != "mw1" || order[1] != "mw2" || order[2] != "handler" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestDefaultRouter_GET(t *testing.T) {
	r := NewDefaultRouter()
	r.GET("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_POST(t *testing.T) {
	r := NewDefaultRouter()
	r.POST("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestDefaultRouter_PUT(t *testing.T) {
	r := NewDefaultRouter()
	r.PUT("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_DELETE(t *testing.T) {
	r := NewDefaultRouter()
	r.DELETE("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestDefaultRouter_PATCH(t *testing.T) {
	r := NewDefaultRouter()
	r.PATCH("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_HEAD(t *testing.T) {
	r := NewDefaultRouter()
	r.HEAD("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("HEAD", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_OPTIONS(t *testing.T) {
	r := NewDefaultRouter()
	r.OPTIONS("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_TRACE(t *testing.T) {
	r := NewDefaultRouter()
	r.TRACE("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("TRACE", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_CONNECT(t *testing.T) {
	r := NewDefaultRouter()
	r.CONNECT("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("CONNECT", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_RegisterRoute_WithMiddlewares(t *testing.T) {
	r := NewDefaultRouter()
	var mwCalled bool
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mwCalled = true
			next.ServeHTTP(w, req)
		})
	}

	r.RegisterRoute(MethodGet, "/with-mw", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, mw)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/with-mw", nil)
	r.ServeHTTP(w, req)

	if !mwCalled {
		t.Fatal("expected middleware to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_RegisterRoutes(t *testing.T) {
	r := NewDefaultRouter()
	routes := Routes{
		"GET /a": func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
		"POST /b": func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusCreated)
		},
	}
	r.RegisterRoutes(routes)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/a", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/b", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestDefaultRouter_HandleFunc(t *testing.T) {
	r := NewDefaultRouter()
	r.HandleFunc("GET /hello", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/hello", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDefaultRouter_Handler(t *testing.T) {
	r := NewDefaultRouter()
	handler := r.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestDefaultRouter_ServeHTTP(t *testing.T) {
	r := NewDefaultRouter()
	var mwCalled bool
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mwCalled = true
			next.ServeHTTP(w, req)
		})
	})

	r.GET("/ping", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ping", nil)
	r.ServeHTTP(w, req)

	if !mwCalled {
		t.Fatal("expected middleware to be called via ServeHTTP")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}