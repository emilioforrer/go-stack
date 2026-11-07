module github.com/emilioforrer/go-stack/examples/coordination

go 1.26.2

require (
	github.com/emilioforrer/go-stack/pkg/coordination v0.0.0
	github.com/jackc/pgx/v5 v5.7.6
	github.com/nats-io/nats-server/v2 v2.10.29
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/nats-io/jwt/v2 v2.7.4 // indirect
	github.com/nats-io/nkeys v0.4.11 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.10.0 // indirect
)

// pkg/coordination is not published as a tagged module yet; resolve it locally.
replace github.com/emilioforrer/go-stack/pkg/coordination => ../
