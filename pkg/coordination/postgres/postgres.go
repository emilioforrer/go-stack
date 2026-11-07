// Package postgres implements coordination.Backend using PostgreSQL row locks.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/emilioforrer/go-stack/pkg/coordination"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Backend struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Backend { return &Backend{db: db} }

var _ coordination.Backend = (*Backend)(nil)

func (b *Backend) Claim(ctx context.Context, req coordination.ClaimRequest, ttl time.Duration) (coordination.Assignment, error) {
	var a coordination.Assignment
	err := b.db.QueryRow(ctx, `WITH candidate AS (
  SELECT slot_id FROM nats_slots WHERE state='available'
  ORDER BY slot_id FOR UPDATE SKIP LOCKED LIMIT 1
 ) UPDATE nats_slots s SET owner_id=$1, generation=s.generation+1,
  state='starting', lease_until=clock_timestamp()+($2::bigint*interval '1 millisecond'),route_url=$3
 FROM candidate c WHERE s.slot_id=c.slot_id
 RETURNING s.slot_id,s.generation,s.owner_id`, req.Owner, ttl.Milliseconds(), req.RouteURL).Scan(&a.Slot, &a.Generation, &a.Owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, coordination.ErrNoSlot
	}
	return a, err
}
func (b *Backend) Renew(ctx context.Context, a coordination.Assignment, ttl time.Duration) error {
	tag, err := b.db.Exec(ctx, `UPDATE nats_slots
 SET lease_until=clock_timestamp()+($4::bigint*interval '1 millisecond')
 WHERE slot_id=$1 AND generation=$2 AND owner_id=$3
   AND state IN ('starting','active') AND lease_until>clock_timestamp()`,
		a.Slot, a.Generation, a.Owner, ttl.Milliseconds())
	return check(tag.RowsAffected(), err)
}
func (b *Backend) Activate(ctx context.Context, a coordination.Assignment) error {
	tag, err := b.db.Exec(ctx, `UPDATE nats_slots SET state='active'
 WHERE slot_id=$1 AND generation=$2 AND owner_id=$3
   AND state='starting' AND lease_until>clock_timestamp()`,
		a.Slot, a.Generation, a.Owner)
	return check(tag.RowsAffected(), err)
}
func (b *Backend) Release(ctx context.Context, a coordination.Assignment) error {
	tag, err := b.db.Exec(ctx, `UPDATE nats_slots
 SET state='available',owner_id=NULL,lease_until=NULL,route_url=NULL
 WHERE slot_id=$1 AND generation=$2 AND owner_id=$3
   AND state IN ('starting','active') AND lease_until>clock_timestamp()`,
		a.Slot, a.Generation, a.Owner)
	return check(tag.RowsAffected(), err)
}
func check(rows int64, err error) error {
	if err != nil {
		return err
	}
	if rows != 1 {
		return coordination.ErrLost
	}
	return nil
}
func (b *Backend) Peers(ctx context.Context, self int) ([]coordination.Peer, error) {
	rows, err := b.db.Query(ctx, `SELECT slot_id,route_url FROM nats_slots
 WHERE slot_id<>$1
   AND state IN ('starting','active')
   AND lease_until>clock_timestamp()
   AND route_url IS NOT NULL
 ORDER BY slot_id`, self)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []coordination.Peer
	for rows.Next() {
		var p coordination.Peer
		if err = rows.Scan(&p.Slot, &p.RouteURL); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (b *Backend) QuarantineExpired(ctx context.Context) (int64, error) {
	tag, err := b.db.Exec(ctx, `UPDATE nats_slots SET state='suspect' WHERE state IN ('starting','active') AND lease_until<=clock_timestamp()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
