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

type Update struct {
	*Client
	XLink
}

type UpdateChan chan Update
type StatsChan chan Stats

// Connect opens the connection to the xlink websocket. It is blocking!
// You MUST read from updateChan and statsChan.
// If you cancel the context, the connection is closed.
func (c *Client) Connect(ctx context.Context, updateChan UpdateChan, statsChan StatsChan) error {
	responseChan := make(jsonrpc.Subscription)
	statisticsChan := make(jsonrpc.Subscription)

	var wg sync.WaitGroup

	withCancel, cancel := context.WithCancel(ctx)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-withCancel.Done():
				return
			case update := <-responseChan:
				var fullXLink XLink
				if err := json.Unmarshal(update.Params, &fullXLink); err != nil {
					c.logger.Error("failed to unmarshal update message", slog.Any("error", err))
					return
				}
				updateChan <- Update{
					Client: c,
					XLink:  fullXLink,
				}
			case stats := <-statisticsChan:
				var rawStats Stats
				if err := json.Unmarshal(stats.Params, &rawStats); err != nil {
					c.logger.Error("failed to unmarshal statistics message", slog.Any("error", err))
					return
				}
				statsChan <- rawStats
			}
		}
	}()

	c.logger.Info("connect to xlink")
	err := c.connect(ctx, responseChan, statisticsChan)
	if err != nil {
		c.logger.Error("unexpected closed connection", slog.Any("error", err))
	} else {
		c.logger.Info("connection closed")
	}
	cancel()
	wg.Wait()
	return err
}

func (c *Client) connect(ctx context.Context, responseChan jsonrpc.Subscription, statsChan jsonrpc.Subscription) error {
	c.jrpc.SubscribeMethod(ctx, "systems.full", responseChan)
	c.jrpc.SubscribeMethod(ctx, "systems.update", responseChan)
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
