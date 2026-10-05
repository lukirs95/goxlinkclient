package xlinkclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"

	jsonrpc "github.com/lukirs95/gojsonrpc/v2"
)

// readLimit is the maximum size of a message from the device. It must hold
// unexpectedly large history blocks of systems.localStatsHistory (600 KB).
const readLimit = 1 << 20

var (
	// ErrNoCredentials is returned by Run if WithCredentials was not given.
	ErrNoCredentials = errors.New("xlinkclient: no credentials configured")
	// ErrAlreadyRunning is returned by Run if the client is already running.
	ErrAlreadyRunning = errors.New("xlinkclient: client is already running")
)

// Update is a snapshot of a system together with the client that produced it.
// The client can be used to send requests to the system.
type Update struct {
	Client *Client
	System System
}

// StatsUpdate is a statistics message together with the client that produced
// it.
type StatsUpdate struct {
	Client *Client
	Stats  Stats
}

// Option configures a Client.
type Option func(*Client)

// WithCredentials sets the user and password used to log in. It is required.
func WithCredentials(user, password string) Option {
	return func(c *Client) {
		c.user, c.password, c.hasCredentials = user, password, true
	}
}

// WithLogger sets the logger. By default nothing is logged.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithUpdates sets the channel that receives a snapshot of the system after
// every change. The channel may be shared by several clients. If the consumer
// is slow, it receives the most recent snapshot of each client; intermediate
// snapshots are dropped, but the connection is never blocked.
func WithUpdates(ch chan<- Update) Option {
	return func(c *Client) {
		c.updatesOut = ch
	}
}

// WithStats sets the channel that receives statistics. Like the updates
// channel it may be shared, and a slow consumer receives the most recent
// statistics of each client.
func WithStats(ch chan<- StatsUpdate) Option {
	return func(c *Client) {
		c.statsOut = ch
	}
}

// Client is a connection to a single VideoXLink system. Its methods are safe
// for concurrent use.
type Client struct {
	addr           string
	user           string
	password       string
	hasCredentials bool
	logger         *slog.Logger
	updatesOut     chan<- Update
	statsOut       chan<- StatsUpdate
	jrpc           *jsonrpc.Client
	running        atomic.Bool
	ready          atomic.Bool

	mu        sync.Mutex
	systemID  SystemID
	snapshot  System
	hasState  bool
	readyWait chan struct{} // closed once the client is ready
}

// New creates a client for the system at addr (host or host:port). The
// connection is opened by Run.
func New(addr string, opts ...Option) *Client {
	c := &Client{
		addr:      addr,
		jrpc:      jsonrpc.NewClient(),
		readyWait: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.logger == nil {
		c.logger = slog.New(slog.DiscardHandler)
	}
	c.logger = c.logger.With(slog.String("system", addr))
	c.jrpc.SetReadLimit(readLimit)
	return c
}

// Addr returns the address the client connects to.
func (c *Client) Addr() string {
	return c.addr
}

// Ready reports whether the client is connected and authenticated.
func (c *Client) Ready() bool {
	return c.ready.Load()
}

// WaitReady blocks until the client is connected and authenticated or ctx is
// done.
func (c *Client) WaitReady(ctx context.Context) error {
	c.mu.Lock()
	wait := c.readyWait
	c.mu.Unlock()
	select {
	case <-wait:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SystemID returns the ID of the system, or "" before the first connection.
func (c *Client) SystemID() SystemID {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.systemID
}

// Snapshot returns the most recent state of the system. ok is false until the
// first systems.full message has been received.
func (c *Client) Snapshot() (sys System, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshot, c.hasState
}

func (c *Client) setReady(ready bool) {
	c.ready.Store(ready)
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.readyWait:
		if !ready {
			c.readyWait = make(chan struct{})
		}
	default:
		if ready {
			close(c.readyWait)
		}
	}
}

// Run connects to the system and processes messages until ctx is done or the
// connection is closed. It may be called again after it returned, e.g. to
// reconnect, but not concurrently.
func (c *Client) Run(ctx context.Context) error {
	if !c.hasCredentials {
		return ErrNoCredentials
	}
	if !c.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer c.running.Store(false)

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	var (
		wg      sync.WaitGroup
		updates *latest[Update]
		stats   *latest[StatsUpdate]
		subs    = newSubscriptions()
	)
	if c.updatesOut != nil {
		updates = newLatest(c.updatesOut)
		wg.Add(1)
		go func() { defer wg.Done(); updates.run(ctx) }()
	}
	if c.statsOut != nil {
		stats = newLatest(c.statsOut)
		wg.Add(1)
		go func() { defer wg.Done(); stats.run(ctx) }()
	}

	// systems.localStatsHistory is deliberately not subscribed; gojsonrpc
	// drops notifications without subscriber.
	for method, ch := range subs.byMethod() {
		c.jrpc.Subscribe(ctx, method, ch)
	}
	defer func() {
		for method := range subs.byMethod() {
			_ = c.jrpc.Unsubscribe(method)
		}
	}()

	wg.Add(2)
	go func() {
		defer wg.Done()
		c.readLoop(ctx, subs, updates, stats)
	}()
	go func() {
		defer wg.Done()
		if err := c.authenticate(ctx, subs.advice); err != nil {
			cancel(err)
			return
		}
		c.subscribeLocalStats(ctx)
	}()

	c.logger.Info("connecting")
	err := c.jrpc.Connect(ctx, fmt.Sprintf("ws://%s/jsonrpc", c.addr), nil)
	authErr := context.Cause(ctx)
	cancel(nil)
	wg.Wait()
	c.setReady(false)

	if authErr != nil && !errors.Is(authErr, context.Canceled) && !errors.Is(authErr, context.DeadlineExceeded) {
		c.logger.Error("authentication failed", slog.Any("error", authErr))
		return authErr
	}
	if err != nil {
		c.logger.Error("connection closed unexpectedly", slog.Any("error", err))
		return err
	}
	c.logger.Info("connection closed")
	return nil
}

// subscriptionBuffer is the capacity of every notification channel. gojsonrpc
// drops a notification if its channel is full. The device sends a few messages
// every couple of seconds and readLoop handles each within microseconds, so
// the buffer only has to absorb bursts such as the messages right after
// login. A lost systems.update is detected through dataid and logged.
const subscriptionBuffer = 32

// subscriptions are the notification channels of one connection.
type subscriptions struct {
	full       jsonrpc.Subscription
	delta      jsonrpc.Subscription
	stats      jsonrpc.Subscription
	localStats jsonrpc.Subscription
	advice     jsonrpc.Subscription
}

func newSubscriptions() subscriptions {
	return subscriptions{
		full:       make(jsonrpc.Subscription, subscriptionBuffer),
		delta:      make(jsonrpc.Subscription, subscriptionBuffer),
		stats:      make(jsonrpc.Subscription, subscriptionBuffer),
		localStats: make(jsonrpc.Subscription, subscriptionBuffer),
		advice:     make(jsonrpc.Subscription, subscriptionBuffer),
	}
}

func (s subscriptions) byMethod() map[jsonrpc.Method]jsonrpc.Subscription {
	return map[jsonrpc.Method]jsonrpc.Subscription{
		notifySystemsFull:    s.full,
		notifySystemsUpdate:  s.delta,
		notifySystemsStats:   s.stats,
		notifyLocalStats:     s.localStats,
		notifyAuthentication: s.advice,
	}
}

// localStatsParams subscribes to systems.localStats. The device always sends
// a history first; batch and max limit it to a single small block. With the
// web UI's values (60/600) the history blocks exceed 600 KB each.
var localStatsParams = map[string]any{"sysid": "local", "batch": 1, "max": 1}

// subscribeLocalStats requests unit and interface statistics. Firmware 1.7 is
// not supported; if the request fails, only the system health is delivered.
func (c *Client) subscribeLocalStats(ctx context.Context) {
	if err := c.call(ctx, methodLocalStats, localStatsParams); err != nil && ctx.Err() == nil {
		c.logger.Warn("unit statistics are not available", slog.Any("error", err))
	}
}

// readLoop merges system messages into the state, publishes snapshots and
// forwards statistics until ctx is done.
func (c *Client) readLoop(ctx context.Context, subs subscriptions, updates *latest[Update], stats *latest[StatsUpdate]) {
	var (
		st     state
		health Health
		peers  []PeerStats
		// Statistics are sent completely every few seconds, so each invalid
		// value is logged only once per connection.
		reported = map[string]bool{}
	)
	logStatsIssues := func(issues []decodeIssue) {
		for _, issue := range issues {
			if !reported[issue.Key] {
				reported[issue.Key] = true
				c.logger.Warn("ignored invalid statistics value", slog.String("value", issue.String()))
			}
		}
	}

	handleFull := func(n jsonrpc.Notification) {
		if err := st.applyFull(n.Params); err != nil {
			c.logger.Error("failed to apply full state", slog.Any("error", err))
			return
		}
		c.publish(&st, updates)
	}
	handleDelta := func(n jsonrpc.Notification) {
		gap, err := st.applyUpdate(n.Params)
		switch {
		case errors.Is(err, errStaleUpdate):
			c.logger.Debug("ignored stale update")
			return
		case err != nil:
			c.logger.Error("failed to apply update", slog.Any("error", err))
			return
		case gap:
			c.logger.Warn("updates were lost, state may be incomplete until reconnect")
		}
		c.publish(&st, updates)
	}
	handleSystemStats := func(n jsonrpc.Notification) {
		h, p, issues, err := decodeSystemStats(n.Params)
		if err != nil {
			c.logger.Error("failed to decode system statistics", slog.Any("error", err))
			return
		}
		logStatsIssues(issues)
		health, peers = h, p
	}
	handleLocalStats := func(n jsonrpc.Notification) {
		ls, issues, err := decodeLocalStats(n.Params)
		if err != nil {
			c.logger.Error("failed to decode statistics", slog.Any("error", err))
			return
		}
		logStatsIssues(issues)
		if stats != nil {
			stats.put(StatsUpdate{Client: c, Stats: Stats{
				System:     ls.System,
				Time:       ls.Time,
				Health:     health,
				Interfaces: ls.Interfaces,
				Encoders:   ls.Encoders,
				Decoders:   ls.Decoders,
				Peers:      slices.Clone(peers),
			}})
		}
	}

	// Each method has its own channel and select picks among ready channels
	// at random, so the arrival order across methods is lost. A pending
	// systems.full is therefore applied before an update, and pending system
	// health before unit statistics. Updates older than the state are ignored
	// by applyUpdate.
	for {
		select {
		case <-ctx.Done():
			return
		case n := <-subs.full:
			handleFull(n)
		case n := <-subs.delta:
			drain(subs.full, handleFull)
			handleDelta(n)
		case n := <-subs.stats:
			handleSystemStats(n)
		case n := <-subs.localStats:
			drain(subs.stats, handleSystemStats)
			handleLocalStats(n)
		}
	}
}

// drain handles every notification already waiting in ch without blocking.
func drain(ch jsonrpc.Subscription, handle func(jsonrpc.Notification)) {
	for {
		select {
		case n := <-ch:
			handle(n)
		default:
			return
		}
	}
}

// publish stores a snapshot of st and hands it to the updates channel. Values
// that could not be decoded are logged.
func (c *Client) publish(st *state, updates *latest[Update]) {
	sys, issues := st.snapshot()
	for _, issue := range issues {
		c.logger.Warn("ignored invalid value from device", slog.String("value", issue.String()))
	}

	c.mu.Lock()
	c.snapshot, c.hasState = sys, true
	c.mu.Unlock()

	if updates != nil {
		// The stored snapshot and the delivered one must not share slices.
		delivered, _ := st.snapshot()
		updates.put(Update{Client: c, System: delivered})
	}
}
