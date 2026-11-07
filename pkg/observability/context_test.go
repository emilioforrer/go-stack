package observability

import (
	"context"
	"testing"
)

func TestWithRequestID_and_RequestID(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-abc")
	got, ok := RequestID(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "req-abc" {
		t.Fatalf("got %q, want %q", got, "req-abc")
	}
}

func TestRequestID_missing(t *testing.T) {
	_, ok := RequestID(context.Background())
	if ok {
		t.Fatal("expected ok=false for missing request_id")
	}
}

func TestWithUserID_and_UserID(t *testing.T) {
	ctx := WithUserID(context.Background(), "user-xyz")
	got, ok := UserID(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "user-xyz" {
		t.Fatalf("got %q, want %q", got, "user-xyz")
	}
}

func TestUserID_missing(t *testing.T) {
	_, ok := UserID(context.Background())
	if ok {
		t.Fatal("expected ok=false for missing user_id")
	}
}
