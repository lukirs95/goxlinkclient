package xlinkclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	jsonrpc "github.com/lukirs95/gojsonrpc/v2"
)

// readLimit is the maximum size of a message from the device. It must hold
// unexpectedly large history blocks of systems.localStatsHistory (600 KB).
const readLimit = 1 << 20

// requestTimeout is how long a request waits for its response, as in the web
// UI. Requests for units of remote systems are forwarded by the local system
// and take up to a few seconds.
const requestTimeout = 5 * time.Second

var (
	// ErrNoCredentials is returned by Run if WithCredentials was not given.
	ErrNoCredentials = errors.New("xlinkclient: no credentials configured")
	// ErrAlreadyRunning is returned by Run if the client is already running.
	ErrAlreadyRunning = errors.New("xlinkclient: client is already running")
	// ErrUpdatesLost is returned by Run if messages of the device were lost.
	// The device only sends the complete state when a connection is opened,
	// so Run ends the connection; the next Run delivers a new snapshot.
	ErrUpdatesLost = errors.New("xlinkclient: updates were lost")
)

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

// WithUpdates sets the channel that receives the state of the system: an
// UpdateSnapshot with the complete system whenever a connection is opened,
// afterwards one Update per element that was added, changed or removed,
// carrying the complete element. The channel may be shared by several
// clients. The connection is never blocked: if the consumer is slow, a
// pending update is replaced by a newer one of the same element, and a
// snapshot replaces the pending updates of its system. Updates still pending
// when Run returns are dropped; the next Run starts with a new snapshot.
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
	readyWait chan struct{} // closed once the client is ready

	// stateMu guards state, which readLoop modifies and Snapshot reads.
	stateMu sync.Mutex
	state   *state
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
	c.jrpc.SetRequestTimeout(requestTimeout)
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
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.state == nil || !c.state.loaded {
		return System{}, false
	}
	sys, _ = c.state.snapshot()
	return sys, true
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

	parent := ctx
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	var (
		wg      sync.WaitGroup
		updates *updateQueue
		stats   *latest[StatsUpdate]
		subs    = newSubscriptions()
	)
	if c.updatesOut != nil {
		updates = newUpdateQueue(c.updatesOut)
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

	// connClosed is closed when Connect returns, authDone when the
	// authentication has finished.
	connClosed, authDone := make(chan struct{}), make(chan struct{})
	// readLoop tells subscribeUnits which units to subscribe and receives
	// the responses.
	want, results := make(chan map[UnitID]bool, 1), make(chan configResult, subscriptionBuffer)
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.readLoop(ctx, cancel, subs, updates, stats, want, results)
	}()
	go func() {
		defer wg.Done()
		err := c.authenticate(ctx, subs.advice, connClosed)
		if err != nil {
			cancel(err)
		}
		close(authDone)
		if err == nil {
			c.subscribeLocalStats(ctx)
			c.subscribeUnits(ctx, want, results)
		}
	}()

	c.logger.Info("connecting")
	err := c.jrpc.Connect(ctx, fmt.Sprintf("ws://%s/jsonrpc", c.addr), nil)
	close(connClosed)
	// The device may close the connection right after rejecting the login,
	// before the response has been handled. The authentication result
	// decides which error is returned.
	<-authDone
	cause := context.Cause(ctx)
	cancel(nil)
	wg.Wait()
	c.setReady(false)

	switch {
	case errors.Is(cause, ErrUpdatesLost):
		c.logger.Warn("updates were lost, connection closed to resynchronize")
		return cause
	case cause != nil && !errors.Is(cause, context.Canceled) && !errors.Is(cause, context.DeadlineExceeded):
		c.logger.Error("authentication failed", slog.Any("error", cause))
		return cause
	}
	// When the caller ends Run, closing the connection may surface as a read
	// error instead of a context error.
	if err != nil && parent.Err() == nil {
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
	unitUpdate jsonrpc.Subscription
	advice     jsonrpc.Subscription
}

func newSubscriptions() subscriptions {
	return subscriptions{
		full:       make(jsonrpc.Subscription, subscriptionBuffer),
		delta:      make(jsonrpc.Subscription, subscriptionBuffer),
		stats:      make(jsonrpc.Subscription, subscriptionBuffer),
		localStats: make(jsonrpc.Subscription, subscriptionBuffer),
		unitUpdate: make(jsonrpc.Subscription, subscriptionBuffer),
		advice:     make(jsonrpc.Subscription, subscriptionBuffer),
	}
}

func (s subscriptions) byMethod() map[jsonrpc.Method]jsonrpc.Subscription {
	return map[jsonrpc.Method]jsonrpc.Subscription{
		notifySystemsFull:    s.full,
		notifySystemsUpdate:  s.delta,
		notifySystemsStats:   s.stats,
		notifyLocalStats:     s.localStats,
		notifyUnitUpdate:     s.unitUpdate,
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

// readLoop merges system messages and unit configurations into the state,
// delivers the changed elements and forwards statistics until ctx is done.
// It hands the units to subscribe to want and ends the connection through
// cancel if messages were lost.
func (c *Client) readLoop(ctx context.Context, cancel context.CancelCauseFunc, subs subscriptions,
	updates *updateQueue, stats *latest[StatsUpdate], want chan map[UnitID]bool, results <-chan configResult) {
	var (
		st     = &state{configs: configs{}}
		wanted map[UnitID]bool
		system systemStats
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
	logIssues := func(issues []decodeIssue) {
		for _, issue := range issues {
			c.logger.Warn("ignored invalid value from device", slog.String("value", issue.String()))
		}
	}

	// deliver hands updates to the consumer. If units were added, removed or
	// relinked, the subscriptions are brought in line with the state.
	deliver := func(list []Update) {
		resubscribe := false
		for i := range list {
			switch list[i].Kind {
			case UpdateSnapshot, UpdateEncoder, UpdateDecoder, UpdateSRTEncoder, UpdateSRTDecoder,
				UpdateNDIEncoder, UpdateNDIDecoder:
				resubscribe = true
			}
			list[i].Client, list[i].System = c, c.SystemID()
		}
		if updates != nil {
			updates.put(list...)
		}
		if !resubscribe {
			return
		}
		c.stateMu.Lock()
		ids := st.subscribable()
		changed := !maps.Equal(ids, wanted)
		if changed {
			for id := range st.configs {
				if !ids[id] {
					delete(st.configs, id)
				}
			}
			for id := range ids {
				st.configs.expect(id)
			}
		}
		c.stateMu.Unlock()
		if changed {
			wanted = ids
			setLatest(want, ids)
		}
	}
	// lost ends the connection; the next Run starts with a new snapshot.
	lost := func(what string) {
		c.logger.Warn(what + " were lost")
		cancel(ErrUpdatesLost)
	}

	handleFull := func(n jsonrpc.Notification) {
		c.stateMu.Lock()
		err := st.applyFull(n.Params)
		var (
			sys    System
			issues []decodeIssue
		)
		if err == nil {
			sys, issues = st.snapshot()
			c.state = st
		}
		c.stateMu.Unlock()
		if err != nil {
			c.logger.Error("failed to apply full state", slog.Any("error", err))
			return
		}
		logIssues(issues)
		deliver([]Update{{Kind: UpdateSnapshot, Snapshot: &sys}})
	}
	handleDelta := func(n jsonrpc.Notification) {
		var ic collector
		c.stateMu.Lock()
		list, gap, err := st.applyUpdate(&ic, n.Params)
		c.stateMu.Unlock()
		logIssues(ic.issues)
		switch {
		case errors.Is(err, errStaleUpdate):
			c.logger.Debug("ignored stale update")
			return
		case err != nil:
			c.logger.Error("failed to apply update", slog.Any("error", err))
			return
		}
		deliver(list)
		if gap {
			lost("system updates")
		}
	}
	handleConfig := func(r configResult) {
		var ic collector
		c.stateMu.Lock()
		list, err := st.applyUnitSubscribe(&ic, r.raw)
		c.stateMu.Unlock()
		logIssues(ic.issues)
		if err != nil {
			c.logger.Error("failed to apply unit configuration", slog.String("unit", string(r.id)), slog.Any("error", err))
			return
		}
		deliver(list)
	}
	handleUnitUpdate := func(n jsonrpc.Notification) {
		var ic collector
		c.stateMu.Lock()
		list, gap, err := st.applyUnitUpdate(&ic, n.Params)
		c.stateMu.Unlock()
		logIssues(ic.issues)
		switch {
		case errors.Is(err, errStaleUpdate), errors.Is(err, errUnknownUnit):
			c.logger.Debug("ignored unit update", slog.Any("reason", err))
			return
		case err != nil:
			c.logger.Error("failed to apply unit update", slog.Any("error", err))
			return
		}
		deliver(list)
		if gap {
			lost("unit updates")
		}
	}
	handleSystemStats := func(n jsonrpc.Notification) {
		ss, issues, err := decodeSystemStats(n.Params)
		if err != nil {
			c.logger.Error("failed to decode system statistics", slog.Any("error", err))
			return
		}
		logStatsIssues(issues)
		system = ss
	}
	handleLocalStats := func(n jsonrpc.Notification) {
		ls, issues, err := decodeLocalStats(n.Params)
		if err != nil {
			c.logger.Error("failed to decode statistics", slog.Any("error", err))
			return
		}
		logStatsIssues(issues)
		merged, issues := mergeStats(ls, system)
		logStatsIssues(issues)
		if stats != nil {
			stats.put(StatsUpdate{Client: c, Stats: merged})
		}
	}

	// Each method has its own channel and select picks among ready channels
	// at random, so the arrival order across methods is lost. A pending
	// systems.full is therefore applied before an update, and pending
	// systems.stats before systems.localStats. Updates older than the state
	// are ignored by applyUpdate.
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
		case r := <-results:
			handleConfig(r)
		case n := <-subs.unitUpdate:
			handleUnitUpdate(n)
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
