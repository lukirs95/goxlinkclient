package xlinkclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	jsonrpc "github.com/lukirs95/gojsonrpc"
)

type clientOption func(c *Client)

// WithLogger is an option you can provide to you use your own slog logger.
func WithLogger(logger *slog.Logger) clientOption {
	return func(c *Client) {
		c.logger = logger.With(slog.String("system", c.ip))
	}
}

type Client struct {
	logger   *slog.Logger
	ip       string
	jrpc     *jsonrpc.JsonRPC
	authKey  string
	systemId string
	ready    atomic.Bool
}

// NewClient creates a xlink client. The Client handles the websocket connection.
func NewClient(ip string, opts ...clientOption) *Client {
	c := &Client{
		jrpc:    jsonrpc.NewJsonRPC(),
		authKey: "",
		ready:   atomic.Bool{},
		ip:      ip,
	}

	for _, option := range opts {
		option(c)
	}

	if c.logger == nil {
		c.logger = slog.New(&NullLogHandler{})
	}

	c.jrpc.SetReadLimit(32768 << 2)
	return c
}

type UpdateChan chan System
type StatsChan chan Stats

// Connect opens the connection to the xlink websocket. It is blocking!
// You MUST read from updateChan and statsChan. Every message from the device
// produces a complete snapshot of the system on updateChan.
// If you cancel the context, the connection is closed.
func (c *Client) Connect(ctx context.Context, updateChan UpdateChan, statsChan StatsChan) error {
	fullChan := make(jsonrpc.Subscription)
	deltaChan := make(jsonrpc.Subscription)
	statisticsChan := make(jsonrpc.Subscription)

	var wg sync.WaitGroup

	withCancel, cancel := context.WithCancel(ctx)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var st state
		for {
			select {
			case <-withCancel.Done():
				return
			case full := <-fullChan:
				if err := st.applyFull(full.Params); err != nil {
					c.logger.Error("failed to apply full state", slog.Any("error", err))
					continue
				}
				c.publish(withCancel, &st, updateChan)
			case delta := <-deltaChan:
				gap, err := st.applyUpdate(delta.Params)
				if err != nil {
					c.logger.Error("failed to apply update", slog.Any("error", err))
					continue
				}
				if gap {
					c.logger.Warn("updates were lost, state may be incomplete until reconnect")
				}
				c.publish(withCancel, &st, updateChan)
			case stats := <-statisticsChan:
				var rawStats Stats
				if err := json.Unmarshal(stats.Params, &rawStats); err != nil {
					c.logger.Error("failed to unmarshal statistics message", slog.Any("error", err))
					continue
				}
				select {
				case statsChan <- rawStats:
				case <-withCancel.Done():
					return
				}
			}
		}
	}()

	c.logger.Info("connect to xlink")
	err := c.connect(ctx, fullChan, deltaChan, statisticsChan)
	if err != nil {
		c.logger.Error("unexpected closed connection", slog.Any("error", err))
	} else {
		c.logger.Info("connection closed")
	}
	cancel()
	wg.Wait()
	return err
}

// publish sends a snapshot of st to updateChan and logs values that could not
// be decoded.
func (c *Client) publish(ctx context.Context, st *state, updateChan UpdateChan) {
	sys, issues := st.snapshot()
	for _, issue := range issues {
		c.logger.Warn("ignored invalid value from device", slog.String("value", issue.String()))
	}
	select {
	case updateChan <- sys:
	case <-ctx.Done():
	}
}

func (c *Client) connect(ctx context.Context, fullChan, deltaChan, statsChan jsonrpc.Subscription) error {
	c.jrpc.SubscribeMethod(ctx, "systems.full", fullChan)
	c.jrpc.SubscribeMethod(ctx, "systems.update", deltaChan)
	c.jrpc.SubscribeMethod(ctx, "systems.stats", statsChan)
	notifyAuth := make(jsonrpc.Subscription)
	c.jrpc.SubscribeMethod(ctx, "notify.auth", notifyAuth)

	withTimeout, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	go c.asyncAuthenticate(withTimeout, notifyAuth)

	endpoint := fmt.Sprintf("ws://%s/jsonrpc", c.ip)
	err := c.jrpc.Connect(ctx, endpoint, nil)

	c.jrpc.UnsubscribeMethod("systems.full")
	c.jrpc.UnsubscribeMethod("systems.update")
	c.jrpc.UnsubscribeMethod("systems.stats")
	c.jrpc.UnsubscribeMethod("notify.auth")

	return err
}

// Ready returns true if the Client is connected to the system.
func (c *Client) Ready() bool {
	return c.ready.Load()
}
