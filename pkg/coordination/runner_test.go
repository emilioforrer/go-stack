package coordination

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeBackend struct {
	mu       sync.Mutex
	assigned bool
	released bool
	renews   int
}

func (b *fakeBackend) Claim(context.Context, ClaimRequest, time.Duration) (Assignment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.assigned {
		return Assignment{}, ErrNoSlot
	}
	b.assigned = true
	return Assignment{Slot: 1, Generation: 3, Owner: "test"}, nil
}
func (b *fakeBackend) Renew(context.Context, Assignment, time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.renews++
	return nil
}
func (b *fakeBackend) Activate(context.Context, Assignment) error { return nil }
func (b *fakeBackend) Release(context.Context, Assignment) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.released = true
	return nil
}
func (b *fakeBackend) Peers(context.Context, int) ([]Peer, error)       { return nil, nil }
func (b *fakeBackend) QuarantineExpired(context.Context) (int64, error) { return 0, nil }

type fakeService struct {
	stopped  bool
	stopErr  error
	startErr error
}

func (s *fakeService) Start(context.Context, Assignment, []Peer) error { return s.startErr }
func (s *fakeService) Stop(context.Context) error                      { s.stopped = true; return s.stopErr }
func TestStopBeforeRelease(t *testing.T) {
	b := &fakeBackend{}
	svc := &fakeService{}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	r := Runner{Backend: b, LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}
	if err := r.RunOnce(ctx, ClaimRequest{Owner: "test"}, svc); err != nil {
		t.Fatal(err)
	}
	if !svc.stopped || !b.released {
		t.Fatal("expected stop and release")
	}
	if b.renews == 0 {
		t.Fatal("renewal goroutine did not run")
	}
}
func TestNoReleaseOnFailedStop(t *testing.T) {
	b := &fakeBackend{}
	svc := &fakeService{startErr: errors.New("start failed"), stopErr: errors.New("storage still open")}
	r := Runner{Backend: b, LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}
	if err := r.RunOnce(context.Background(), ClaimRequest{Owner: "test"}, svc); err == nil {
		t.Fatal("expected error")
	}
	if b.released {
		t.Fatal("released slot with unconfirmed storage shutdown")
	}
}
