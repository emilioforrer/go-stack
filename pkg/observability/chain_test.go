package observability

import (
	"context"
	"testing"
)

func TestChain_empty(t *testing.T) {
	called := false
	fn := func(ctx context.Context) (string, error) {
		called = true
		return "ok", nil
	}

	res, err := Chain[string]()(fn)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler not called")
	}
	if res != "ok" {
		t.Fatalf("got %q, want %q", res, "ok")
	}
}

func TestChain_order(t *testing.T) {
	var order []int

	mw := func(id int) Middleware[string] {
		return func(next Handler[string]) Handler[string] {
			return func(ctx context.Context) (string, error) {
				order = append(order, id)
				return next(ctx)
			}
		}
	}

	fn := func(ctx context.Context) (string, error) {
		order = append(order, 0)
		return "done", nil
	}

	_, _ = Chain(mw(1), mw(2), mw(3))(fn)(context.Background())

	if len(order) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(order))
	}
	// Middlewares should execute in order: 1, 2, 3, then handler (0)
	expected := []int{1, 2, 3, 0}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("order[%d] = %d, want %d", i, order[i], v)
		}
	}
}

func TestChain_single(t *testing.T) {
	called := false
	mw := func(next Handler[int]) Handler[int] {
		return func(ctx context.Context) (int, error) {
			called = true
			return next(ctx)
		}
	}

	fn := func(ctx context.Context) (int, error) { return 42, nil }

	res, err := Chain(mw)(fn)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("middleware not called")
	}
	if res != 42 {
		t.Fatalf("got %d, want 42", res)
	}
}
