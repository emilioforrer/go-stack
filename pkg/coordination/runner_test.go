package coordination

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// errTransientRenewal simulates a backend outage that is neither ErrLost nor
// ErrNoSlot: the runner must keep renewing while the lease deadline is far
// ahead and trip the deadline check once it is not. It guards the intermittent
// renewal stall observed during the docker eval (see example EVAL.md).
var errTransientRenewal = errors.New("transient renewal outage")

type fakeBackend struct {
	mu       sync.Mutex
	assigned bool
	released bool
	renews   int

	// failure-injection knobs (zero values = healthy backend)
	renewFn  func(call int) error
	peersErr error

	// largest positive headroom observed on a Renew context deadline
	maxRenewUntil time.Duration
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

func (b *fakeBackend) Renew(ctx context.Context, _ Assignment, _ time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.renews++
	if d, ok := ctx.Deadline(); ok {
		if in := time.Until(d); in > 0 && in > b.maxRenewUntil {
			b.maxRenewUntil = in
		}
	}
	if b.renewFn != nil {
		return b.renewFn(b.renews)
	}
	return nil
}

func (b *fakeBackend) Activate(context.Context, Assignment) error { return nil }

func (b *fakeBackend) Release(context.Context, Assignment) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.released = true
	return nil
}

func (b *fakeBackend) Peers(context.Context, int) ([]Peer, error) {
	if b.peersErr != nil {
		return nil, b.peersErr
	}
	return nil, nil
}

func (b *fakeBackend) QuarantineExpired(context.Context) (int64, error) { return 0, nil }

type fakeService struct {
	stopped  bool
	started  bool
	stopErr  error
	startErr error
}

func (s *fakeService) Start(_ context.Context, _ Assignment, _ []Peer) error {
	s.started = true
	return s.startErr
}

func (s *fakeService) Stop(context.Context) error {
	s.stopped = true
	return s.stopErr
}

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
		t.Fatal("release must be skipped when the service stop is unconfirmed")
	}
	if !svc.started {
		t.Fatal("service must have been started")
	}
}

// --- configuration validation ---

func TestValidateRejectsInvalidConfigs(t *testing.T) {
	cases := []struct {
		name string
		r    Runner
	}{
		{"nil backend", Runner{LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}},
		{"zero lease ttl", Runner{Backend: &fakeBackend{}, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}},
		{"zero renew every", Runner{Backend: &fakeBackend{}, LeaseTTL: time.Second, StopTimeout: 100 * time.Millisecond}},
		{"zero stop timeout", Runner{Backend: &fakeBackend{}, LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond}},
		{"renew every exceeds ttl", Runner{Backend: &fakeBackend{}, LeaseTTL: time.Second, RenewEvery: 2 * time.Second, StopTimeout: 100 * time.Millisecond}},
		{"renew+stop consumes ttl", Runner{Backend: &fakeBackend{}, LeaseTTL: time.Second, RenewEvery: time.Second, StopTimeout: 100 * time.Millisecond}},
		{"renew+stop==ttl not allowed", Runner{Backend: &fakeBackend{}, LeaseTTL: time.Second, RenewEvery: 900 * time.Millisecond, StopTimeout: 100 * time.Millisecond}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.r.validate(); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("want ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestValidateAcceptsHealthyConfig(t *testing.T) {
	r := Runner{Backend: &fakeBackend{}, LeaseTTL: time.Second, RenewEvery: 100 * time.Millisecond, StopTimeout: 500 * time.Millisecond}
	if err := r.validate(); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestRunOnceRequiresService(t *testing.T) {
	b := &fakeBackend{}
	r := Runner{Backend: b, LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}
	if err := r.RunOnce(context.Background(), ClaimRequest{Owner: "test"}, nil); !errors.Is(err, ErrNilService) {
		t.Fatalf("want ErrNilService, got %v", err)
	}
}

// --- renewal failure paths (the anomaly guards) ---

func TestRenewalLostTripsImmediately(t *testing.T) {
	b := &fakeBackend{renewFn: func(int) error { return ErrLost }}
	svc := &fakeService{}
	r := Runner{Backend: b, LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}

	start := time.Now()
	err := r.RunOnce(context.Background(), ClaimRequest{Owner: "test"}, svc)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrLost) {
		t.Fatalf("want ErrLost, got %v", err)
	}
	if !strings.Contains(err.Error(), "renew slot 1") {
		t.Fatalf("error should identify the slot: %v", err)
	}
	// ErrLost must report on the first tick, not wait for the deadline.
	if elapsed > 500*time.Millisecond {
		t.Fatalf("ErrLost should trip immediately, took %v", elapsed)
	}
	if !svc.stopped || !b.released {
		t.Fatal("expected stop and release after renewal loss")
	}
	if b.renews == 0 {
		t.Fatal("renewal was never attempted")
	}
}

func TestRenewalDeadlineTripsWhenLeaseCannotExtend(t *testing.T) {
	b := &fakeBackend{renewFn: func(int) error { return errTransientRenewal }}
	svc := &fakeService{}
	// deadline trip lands ~StopTimeout+RenewEvery before the TTL drains:
	// 1s - (100ms+20ms) = ~880ms of ticks.
	r := Runner{Backend: b, LeaseTTL: time.Second, RenewEvery: 20 * time.Millisecond, StopTimeout: 100 * time.Millisecond}

	start := time.Now()
	err := r.RunOnce(context.Background(), ClaimRequest{Owner: "test"}, svc)
	elapsed := time.Since(start)

	if !errors.Is(err, errTransientRenewal) {
		t.Fatalf("want transient renewal error, got %v", err)
	}
	if elapsed < 600*time.Millisecond || elapsed > 1200*time.Millisecond {
		t.Fatalf("deadline trip expected ~880ms, took %v", elapsed)
	}
	if !svc.stopped || !b.released {
		t.Fatal("expected stop and release after deadline trip")
	}
}

func TestRenewalTransientFailuresIgnoredWithinDeadline(t *testing.T) {
	b := &fakeBackend{renewFn: func(call int) error {
		if call == 1 {
			return errTransientRenewal
		}
		return nil // recovery: deadline advances, later failures stay ignorable
	}}
	svc := &fakeService{}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	r := Runner{Backend: b, LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}

	start := time.Now()
	err := r.RunOnce(ctx, ClaimRequest{Owner: "test"}, svc)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("transient failure inside the deadline must be ignored, got %v", err)
	}
	// without the deadline advance this run would trip at ~890ms; it must end
	// with the (short) context, proving the successful renewal extended it.
	if elapsed > 400*time.Millisecond {
		t.Fatalf("run should end with the 80ms context, took %v", elapsed)
	}
	if !svc.stopped || !b.released {
		t.Fatal("expected stop and release on clean shutdown")
	}
}

func TestRenewUsesBoundedContext(t *testing.T) {
	b := &fakeBackend{}
	svc := &fakeService{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	r := Runner{Backend: b, LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}

	if err := r.RunOnce(ctx, ClaimRequest{Owner: "test"}, svc); err != nil {
		t.Fatal(err)
	}

	b.mu.Lock()
	headroom := b.maxRenewUntil
	b.mu.Unlock()
	// each Renew call must carry a deadline bounded by RenewEvery, so a hung
	// backend call can never outlive the tick interval. (Later calls may see
	// an already-expired run context, hence the max-positive comparison.)
	if headroom <= 0 || headroom > 10*time.Millisecond+5*time.Millisecond {
		t.Fatalf("renew context headroom want (0, 15ms], got %v", headroom)
	}
}

// --- peers lookup failure ---

func TestRunOnceSurfacesPeersError(t *testing.T) {
	peersErr := errors.New("peers lookup unavailable")
	b := &fakeBackend{peersErr: peersErr}
	svc := &fakeService{}
	r := Runner{Backend: b, LeaseTTL: time.Second, RenewEvery: 10 * time.Millisecond, StopTimeout: 100 * time.Millisecond}

	err := r.RunOnce(context.Background(), ClaimRequest{Owner: "test"}, svc)
	if !errors.Is(err, peersErr) {
		t.Fatalf("want peers error, got %v", err)
	}
	// a failed startup is still stopped before the slot is released.
	if !svc.stopped || !b.released {
		t.Fatal("expected stop and release after peers failure")
	}
	if svc.started {
		t.Fatal("service must not start without peers")
	}
}
