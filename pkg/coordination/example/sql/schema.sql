CREATE TABLE IF NOT EXISTS nats_slots (
  slot_id integer PRIMARY KEY CHECK (slot_id >= 0),
  generation bigint NOT NULL DEFAULT 0,
  owner_id text,
  state text NOT NULL DEFAULT 'available' CHECK (state IN ('available','starting','active','suspect')),
  lease_until timestamptz,
  route_url text,
  CHECK ((state='available' AND owner_id IS NULL AND lease_until IS NULL AND route_url IS NULL)
    OR (state<>'available' AND owner_id IS NOT NULL AND lease_until IS NOT NULL))
);
INSERT INTO nats_slots(slot_id) VALUES (0),(1),(2) ON CONFLICT DO NOTHING;
-- NEVER truncate/drop this table during application upgrades.
