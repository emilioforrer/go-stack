package main

// boot.go wires the coordination demo to the boot lifecycle (pkg/boot).
//
// Providers (registration order defines boot order; Shutdown is reversed):
//  1. PostgresProvider   - opens the *pgxpool.Pool (DATABASE_URL), registers it
//     and the coordination.Backend in the DI container.
//  2. NATSServerProvider - owns a coordination.Runner; its Boot phase starts
//     the claim/retry loop goroutine. Each RunOnce claims a slot (lease TTL
//     30s, renewed every 5s), starts the embedded NATS server for that slot,
//     and, once ready, it is exposed into the container. Instances that
//     cannot claim a slot keep retrying in client-only mode. Shutdown
//     cancels the loop, which makes RunOnce stop the server and release the
//     slot.
//  3. NATSClientProvider - resolves the *nats.Conn container service; it
//     connects as soon as a live server is ready, or reports client-only
//     mode when the instance never claims one.
//
// Container services: *pgxpool.Pool, *postgres.Backend, *Coordinator (claim
// state handle), *embeddedNATS (ready once a slot is claimed; absent on
// client-only instances) and *nats.Conn.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/emilioforrer/go-stack/pkg/coordination"
	pgbackend "github.com/emilioforrer/go-stack/pkg/coordination/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats-server/v2/server"
	natsio "github.com/nats-io/nats.go"
	"github.com/samber/do/v2"
)

const (
	//nolint:gosec // local-dev demo default, overridable via DATABASE_URL
	defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	defaultLeaseTTL    = 30 * time.Second
	defaultRenewEvery  = 5 * time.Second
	defaultStopTimeout = 10 * time.Second
	retryDelay         = 5 * time.Second
	serverWaitTimeout  = 15 * time.Second
	defaultNATSDataDir = "./data"
	defaultClusterName = "app-nats"
	defaultClientPort  = 4222
	defaultRoutePort   = 6222
)

// errNoEmbeddedServer is reported to container consumers when this instance
// never claims a slot (a surplus / client-only replica).
var errNoEmbeddedServer = errors.New("no embedded NATS server became ready (client-only instance)")

// Coordinator exposes the claim state to the rest of the app. The live
// server registers itself when NATS is ready; WaitReady resolves it.
type Coordinator struct {
	mu    sync.RWMutex
	svc   *embeddedNATS
	ready chan struct{}
	once  sync.Once
}

func newCoordinator() *Coordinator {
	return &Coordinator{ready: make(chan struct{})}
}

// register stores the live embedded NATS server and wakes WaitReady callers.
// Called from the service's Start phase on every claim: the latest server
// wins; the *embeddedNATS container factory returns the first ready server
// for the process lifetime (a process-level re-claim starts a NEW server
// instance, but the container keeps serving the first resolved one).
func (c *Coordinator) register(s *embeddedNATS) {
	c.mu.Lock()
	c.svc = s
	c.mu.Unlock()
	c.once.Do(func() { close(c.ready) })
}

// WaitReady blocks until the embedded NATS server is ready or ctx is done.
func (c *Coordinator) WaitReady(ctx context.Context) (*embeddedNATS, bool) {
	select {
	case <-c.ready:
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.svc, true
	case <-ctx.Done():
		return nil, false
	}
}

// PostgresProvider opens the database pool and exposes the slot backend.
type PostgresProvider struct {
	boot.DefaultProvider
	dsn  string
	pool *pgxpool.Pool
}

func NewPostgresProvider() *PostgresProvider { return &PostgresProvider{} }

func (p *PostgresProvider) Register(ctx context.Context, container boot.Container) error {
	p.dsn = env("DATABASE_URL", defaultDatabaseURL)

	pool, err := pgxpool.New(ctx, p.dsn)
	if err != nil {
		return fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("postgres: ping: %w", err)
	}
	p.pool = pool

	do.ProvideValue(container.Injector(), pool)
	do.Provide(container.Injector(), func(i do.Injector) (*pgbackend.Backend, error) {
		pool, err := do.Invoke[*pgxpool.Pool](i)
		if err != nil {
			return nil, fmt.Errorf("postgres: resolve pool: %w", err)
		}
		return pgbackend.New(pool), nil
	})

	slog.Info("postgres provider registered", "dsn", p.dsn)
	return nil
}

func (p *PostgresProvider) Boot(_ context.Context, _ boot.Container) error {
	return nil
}

func (p *PostgresProvider) Shutdown(_ context.Context, _ boot.Container) error {
	if p.pool == nil {
		return nil
	}
	p.pool.Close()
	slog.Info("postgres provider shut down")
	return nil
}

// NATSServerProvider owns the claim/retry loop and the embedded NATS server.
type NATSServerProvider struct {
	boot.DefaultProvider
	runner    coordination.Runner
	request   coordination.ClaimRequest
	routePort int
	coord     *Coordinator
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewNATSServerProvider() *NATSServerProvider { return &NATSServerProvider{} }

func (p *NATSServerProvider) Register(_ context.Context, container boot.Container) error {
	backend, err := do.Invoke[*pgbackend.Backend](container.Injector())
	if err != nil {
		return fmt.Errorf("nats server: resolve backend: %w", err)
	}

	p.routePort = port("NATS_ROUTE_PORT", defaultRoutePort)
	p.runner = coordination.Runner{
		Backend:     backend,
		LeaseTTL:    defaultLeaseTTL,
		RenewEvery:  defaultRenewEvery,
		StopTimeout: defaultStopTimeout,
	}
	p.request = coordination.ClaimRequest{
		Owner:    ownerID(),
		RouteURL: "nats-route://" + net.JoinHostPort(env("POD_IP", "127.0.0.1"), strconv.Itoa(p.routePort)),
	}

	p.coord = newCoordinator()
	do.ProvideValue(container.Injector(), p.coord)

	// Container service: the live embedded NATS server. It becomes available
	// on the first successful claim, and stays absent (the factory reports
	// errNoEmbeddedServer) on instances that never claim a slot.
	do.Provide(container.Injector(), func(_ do.Injector) (*embeddedNATS, error) {
		readyCtx, cancel := context.WithTimeout(context.Background(), serverWaitTimeout)
		defer cancel()
		svc, ok := p.coord.WaitReady(readyCtx)
		if !ok {
			return nil, fmt.Errorf("nats server: %w", errNoEmbeddedServer)
		}
		return svc, nil
	})

	slog.Info("nats server provider registered", "cluster", env("NATS_CLUSTER_NAME", defaultClusterName), "route_port", p.routePort)
	return nil
}

func (p *NATSServerProvider) Boot(_ context.Context, _ boot.Container) error {
	loopCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go p.loop(loopCtx)

	slog.Info("nats server provider bootstrapped", "lease_ttl", defaultLeaseTTL, "renew_every", defaultRenewEvery)
	return nil
}

// loop keeps claiming a slot and running the embedded NATS server until the
// process is shut down. ErrNoSlot is not fatal: with no free slot the
// instance serves as client-only and the loop retries after a delay.
func (p *NATSServerProvider) loop(ctx context.Context) {
	defer close(p.done)

	for ctx.Err() == nil {
		nats := &embeddedNATS{
			routePort: p.routePort,
			ip:        env("POD_IP", "127.0.0.1"),
			dir:       env("NATS_DATA_DIR", defaultNATSDataDir),
			coord:     p.coord,
		}
		err := p.runner.RunOnce(ctx, p.request, nats)
		if ctx.Err() != nil {
			// Shutdown: RunOnce already stopped the server and released the slot.
			return
		}
		switch {
		case errors.Is(err, coordination.ErrNoSlot):
			slog.Info("no NATS slot: running as client-only, retrying", "delay", "5s")
		case err != nil:
			slog.Error("coordinator stopped, retrying", "error", err)
		default:
			slog.Info("NATS server exited, retrying")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryDelay):
		}
	}
}

func (p *NATSServerProvider) Shutdown(ctx context.Context, _ boot.Container) error {
	if p.cancel == nil {
		return nil
	}
	p.cancel()

	select {
	case <-p.done:
		slog.Info("embedded NATS coordinator released its slot")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("nats server: slot release unconfirmed: %w", ctx.Err())
	}
}

// embeddedNATS implements coordination.Service backed by a NATS server started
// inside this process. Start binds the client port, joins the cluster through
// the route port, and seeds JetStream storage per slot; Stop waits for the
// server to quiesce. Once ready it registers itself with the Coordinator.
type embeddedNATS struct {
	ns        *server.Server
	routePort int
	ip, dir   string
	coord     *Coordinator
}

func (s *embeddedNATS) Start(ctx context.Context, a coordination.Assignment, peers []coordination.Peer) error {
	routes := make([]*url.URL, 0, len(peers))
	for _, p := range peers {
		u, e := url.Parse(p.RouteURL)
		if e != nil {
			return e
		}
		routes = append(routes, u)
	}
	opts := &server.Options{
		ServerName:         a.Name(),
		Host:               "0.0.0.0",
		Port:               port("NATS_CLIENT_PORT", defaultClientPort),
		JetStream:          true,
		StoreDir:           filepath.Join(s.dir, a.Name()),
		JetStreamMaxStore:  10 << 30,
		JetStreamMaxMemory: 256 << 20,
		// The app owns process signal handling (signal.NotifyContext) and the
		// graceful shutdown order (service.Stop -> backend.Release). NATS's
		// embedded signal handler (installed by Server.Start; NoSigs is the
		// only opt-out) would race it: on SIGTERM both paths call
		// Server.Shutdown concurrently, which nats-server does not tolerate
		// (shutdownEventing runs before the idempotency guard, so the second
		// entrant panics on close of the already-nulled resetCh, or the
		// handler's os.Exit kills the process before the slot is released).
		NoSigs: true,
		Routes: routes,
	}
	// JetStream cluster mode fatally requires at least one route connection,
	// so the process that claims the first slot (no peers yet) must start a
	// standalone server. It keeps the slot identity; NATS routes are static
	// per start here, so it rejoins as a clustered node on later generations
	// (after release + a fresh claim with active peers).
	if len(routes) > 0 {
		opts.Cluster = server.ClusterOpts{
			Name:      env("NATS_CLUSTER_NAME", defaultClusterName),
			Host:      "0.0.0.0",
			Port:      s.routePort,
			Advertise: net.JoinHostPort(s.ip, strconv.Itoa(s.routePort)),
		}
	}
	ns, e := server.NewServer(opts)
	if e != nil {
		return e
	}
	s.ns = ns
	// Deliberately NOT calling ns.ConfigureLogger(): in embedded (library)
	// mode it also installs nats-server's own process signal handler. The app
	// owns signal handling and the lifecycle log lines.
	ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		ns.Shutdown()
		ns.WaitForShutdown()
		s.ns = nil
		return errNatsNotReady
	}
	s.coord.register(s)
	slog.Info("embedded NATS started", "server", a.Name(), "store_dir", opts.StoreDir, "peers", len(routes))
	return nil
}

var errNatsNotReady = errors.New("NATS failed to become ready")

func (s *embeddedNATS) Stop(ctx context.Context) error {
	if s.ns == nil {
		return nil
	}
	s.ns.Shutdown()
	done := make(chan struct{})
	go func() { s.ns.WaitForShutdown(); close(done) }()
	select {
	case <-done:
		s.ns = nil
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// NATSClientProvider connects a real NATS client to the live server.
type NATSClientProvider struct {
	boot.DefaultProvider
	url  string
	conn *natsio.Conn
}

func NewNATSClientProvider() *NATSClientProvider {
	return &NATSClientProvider{
		url: fmt.Sprintf("nats://127.0.0.1:%d", port("NATS_CLIENT_PORT", defaultClientPort)),
	}
}

func (p *NATSClientProvider) Register(_ context.Context, container boot.Container) error {
	// Container service: the NATS client connection. The factory waits for a
	// live server, connects, and is only invoked by this provider's Boot.
	do.Provide(container.Injector(), func(i do.Injector) (*natsio.Conn, error) {
		svc, err := do.Invoke[*embeddedNATS](i)
		if err != nil {
			return nil, fmt.Errorf("nats client: %w", err)
		}
		conn, err := natsio.Connect(p.url, natsio.Name("go-stack/coordination-example"))
		if err != nil {
			return nil, fmt.Errorf("nats client: connect %s: %w", p.url, err)
		}
		p.conn = conn
		serverName := "nats"
		if svc.ns != nil {
			serverName = svc.ns.Name()
		}
		slog.Info("nats client connected", "addr", p.url, "server", serverName, "server_id", conn.ConnectedServerId())
		return conn, nil
	})
	return nil
}

func (p *NATSClientProvider) Boot(_ context.Context, container boot.Container) error {
	_, err := do.Invoke[*natsio.Conn](container.Injector())
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNoEmbeddedServer):
		slog.Info("no embedded NATS server: running as client-only")
		return nil
	default:
		return fmt.Errorf("nats client: %w", err)
	}
}

func (p *NATSClientProvider) Shutdown(_ context.Context, _ boot.Container) error {
	if p.conn == nil {
		return nil
	}
	if err := p.conn.Drain(); err != nil {
		return fmt.Errorf("nats client: drain: %w", err)
	}
	slog.Info("nats client drained")
	return nil
}
