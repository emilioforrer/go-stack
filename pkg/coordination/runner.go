package coordination

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

var (
	// ErrInvalidConfig is returned when the Runner's timing knobs are inconsistent.
	ErrInvalidConfig = errors.New("invalid runner configuration: RenewEvery + StopTimeout must be < LeaseTTL")
	// ErrNilService is returned when RunOnce is called without a Service.
	ErrNilService = errors.New("nil service")
)

// Service owns the embedded process and its storage. Stop must return only
// after the process has stopped writing. Start must clean up on failure.
type Service interface {
	Start(context.Context, Assignment, []Peer) error
	Stop(context.Context) error
}

// Runner owns a single identity attempt. It does not automatically reclaim
// expired/suspect slots: storage fencing is an external responsibility.
type Runner struct {
	Backend     Backend
	LeaseTTL    time.Duration
	RenewEvery  time.Duration
	StopTimeout time.Duration
}

func (r Runner) validate() error {
	if r.Backend == nil || r.LeaseTTL <= 0 || r.RenewEvery <= 0 || r.StopTimeout <= 0 || r.RenewEvery+r.StopTimeout >= r.LeaseTTL {
		return ErrInvalidConfig
	}
	return nil
}

func (r Runner) RunOnce(ctx context.Context, req ClaimRequest, service Service) error {
	if err := r.validate(); err != nil {
		return err
	}
	if service == nil {
		return ErrNilService
	}
	a, err := r.Backend.Claim(ctx, req, r.LeaseTTL)
	if err != nil {
		return err
	}

	// Renewal starts BEFORE peer lookup/server startup, which may take seconds.
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	renewalDone := make(chan error, 1)
	go r.renew(workCtx, a, renewalDone)

	// A failed startup is still stopped before the slot is released. The
	// backend refuses release of an expired or transferred generation.
	var startErr error
	peers, err := r.Backend.Peers(workCtx, a.Slot)
	if err == nil {
		startErr = service.Start(workCtx, a, peers)
	} else {
		startErr = err
	}
	if startErr == nil {
		startErr = r.Backend.Activate(workCtx, a)
	}
	if startErr == nil {
		select {
		case <-ctx.Done():
			slog.Info("runner: boot context done, stopping slot", "slot", a.Slot)
		case renewalErr := <-renewalDone:
			startErr = renewalErr
			slog.Error("runner: lease renewal failed, stopping slot", "slot", a.Slot, "error", renewalErr)
		}
	}
	cancel()
	// WorkCtx cancellation terminates renewal before final release.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), r.StopTimeout)
	defer stopCancel()
	if stopErr := service.Stop(stopCtx); stopErr != nil {
		return errors.Join(startErr, fmt.Errorf("storage shutdown unconfirmed: slot %d remains allocated: %w", a.Slot, stopErr))
	}
	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), r.StopTimeout)
	defer releaseCancel()
	if releaseErr := r.Backend.Release(releaseCtx, a); releaseErr != nil {
		// On expiry, no reuse occurs: backend keeps the slot assigned/suspect.
		return errors.Join(startErr, fmt.Errorf("release slot %d: %w", a.Slot, releaseErr))
	}
	return startErr
}

func (r Runner) renew(ctx context.Context, a Assignment, result chan<- error) {
	tick := time.NewTicker(r.RenewEvery)
	defer tick.Stop()
	deadline := time.Now().Add(r.LeaseTTL)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			renewCtx, cancel := context.WithTimeout(ctx, r.RenewEvery)
			err := r.Backend.Renew(renewCtx, a, r.LeaseTTL)
			cancel()
			if err == nil {
				deadline = time.Now().Add(r.LeaseTTL)
				continue
			}
			slog.Warn("runner: slot renew failed", "slot", a.Slot, "error", err)
			if errors.Is(err, ErrLost) || time.Until(deadline) <= r.StopTimeout+r.RenewEvery {
				select {
				case result <- fmt.Errorf("renew slot %d: %w", a.Slot, err):
				default:
				}
				cancel()
				return
			}
		}
	}
}
