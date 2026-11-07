// Package coordination defines backend-independent identity leasing for
// embedded distributed services. A slot is a stable logical identity.
package coordination

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNoSlot = errors.New("no free logical slot")
	ErrLost   = errors.New("slot ownership lost")
)

type Assignment struct {
	Slot       int
	Generation int64
	Owner      string
}

func (a Assignment) Name() string { return fmt.Sprintf("nats-%d", a.Slot) }

type Peer struct {
	Slot     int
	RouteURL string
}
type ClaimRequest struct{ Owner, RouteURL string }

// Backend must serialize acquisition, reject stale owner/generation writes,
// and never reallocate an expired lease without independently verified fencing.
type Backend interface {
	Claim(context.Context, ClaimRequest, time.Duration) (Assignment, error)
	Renew(context.Context, Assignment, time.Duration) error
	Activate(context.Context, Assignment) error
	Release(context.Context, Assignment) error
	Peers(context.Context, int) ([]Peer, error)
	QuarantineExpired(context.Context) (int64, error)
}
