package xlinkclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	jsonrpc "github.com/lukirs95/gojsonrpc"
)

// readLimit is the maximum size of a message from the device. systems.full of
// a fully equipped system is well below this.
const readLimit = 128 << 10

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
	jrpc           *jsonrpc.JsonRPC
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
		jrpc:      jsonrpc.NewJsonRPC(),
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
		wg       sync.WaitGroup
		updates  *latest[Update]
		stats    *latest[StatsUpdate]
		fullCh   = make(jsonrpc.Subscription)
		deltaCh  = make(jsonrpc.Subscription)
		statsCh  = make(jsonrpc.Subscription)
		adviceCh = make(jsonrpc.Subscription)
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

	c.jrpc.SubscribeMethod(ctx, notifySystemsFull, fullCh)
	c.jrpc.SubscribeMethod(ctx, notifySystemsUpdate, deltaCh)
	c.jrpc.SubscribeMethod(ctx, notifySystemsStats, statsCh)
	c.jrpc.SubscribeMethod(ctx, notifyAuthentication, adviceCh)
	defer func() {
		for _, m := range []jsonrpc.Method{notifySystemsFull, notifySystemsUpdate, notifySystemsStats, notifyAuthentication} {
			c.jrpc.UnsubscribeMethod(m)
		}
	}()

	// gojsonrpc delivers notifications synchronously from its read loop and
	// does not observe the context while doing so. Every subscription is
	// therefore read until Connect has returned, not just until ctx is done;
	// otherwise a notification arriving during shutdown blocks Connect forever.
	connDone := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		c.readLoop(connDone, fullCh, deltaCh, statsCh, updates, stats)
	}()
	go func() {
		defer wg.Done()
		if err := c.authenticate(ctx, adviceCh); err != nil {
			cancel(err)
		}
		// The device may announce itself again after login.
		for {
			select {
			case <-connDone:
				return
			case <-adviceCh:
			}
		}
	}()

	c.logger.Info("connecting")
	err := c.jrpc.Connect(ctx, fmt.Sprintf("ws://%s/jsonrpc", c.addr), nil)
	authErr := context.Cause(ctx)
	cancel(nil)
	close(connDone)
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

// readLoop merges system messages into the state, publishes snapshots and
// forwards statistics until done is closed.
func (c *Client) readLoop(done <-chan struct{}, fullCh, deltaCh, statsCh jsonrpc.Subscription, updates *latest[Update], stats *latest[StatsUpdate]) {
	var st state
	for {
		select {
		case <-done:
			return
		case full := <-fullCh:
			if err := st.applyFull(full.Params); err != nil {
				c.logger.Error("failed to apply full state", slog.Any("error", err))
				continue
			}
			c.publish(&st, updates)
		case delta := <-deltaCh:
			gap, err := st.applyUpdate(delta.Params)
			if err != nil {
				c.logger.Error("failed to apply update", slog.Any("error", err))
				continue
			}
			if gap {
				c.logger.Warn("updates were lost, state may be incomplete until reconnect")
			}
			c.publish(&st, updates)
		case msg := <-statsCh:
			var s Stats
			if err := json.Unmarshal(msg.Params, &s); err != nil {
				c.logger.Error("failed to decode statistics", slog.Any("error", err))
				continue
			}
			if stats != nil {
				stats.put(StatsUpdate{Client: c, Stats: s})
			}
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
