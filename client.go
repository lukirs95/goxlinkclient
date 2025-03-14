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
	xlink "github.com/lukirs95/goxlinkclient/internal/model"
)

type clientOption func(c *Client)

// WithLogger is an option you can provide to you use your own logger.
func WithLogger(logger *slog.Logger) clientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithReconnect lets you adjust the automatic reconnection if an error accurs.
func WithReconnect(interval time.Duration) clientOption {
	return func(c *Client) {
		c.reconnectDelay = interval
	}
}

// WithPassword lets you adjust the password that is been used to connect to the system
func WithPassword(password string) clientOption {
	return func(c *Client) {
		c.password = password
	}
}

type Client struct {
	logger         *slog.Logger
	reconnectDelay time.Duration
	ip             string
	password       string
	jrpc           *jsonrpc.JsonRPC
	authKey        string
	systemId       string
	ready          atomic.Bool
}

// NewClient creates a new instance of the client. The Client handles the connection.
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

	if c.password == "" {
		c.password = "123456!"
	}

	if c.logger == nil {
		c.logger = slog.New(&NullLogHandler{})
	}

	if c.reconnectDelay == 0 {
		c.reconnectDelay = time.Second * 10
	}

	authMessage.Password = c.password
	c.jrpc.OnDisconnect = c.onDisconnect
	c.jrpc.SetReadLimit(32768 << 2)
	return c
}

type UpdateChan chan XLink
type StatsChan chan Stats

// Connect attempts to connect to the system. It is blocking!
// If you cancle the context, the connection is closed.
func (c *Client) Connect(ctx context.Context, updateChan UpdateChan, statsChan StatsChan) {
	responseChan := make(jsonrpc.Subscription)
	statisticsChan := make(jsonrpc.Subscription)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case update := <-responseChan:
				var fullXLink xlink.XLinkRaw
				if err := json.Unmarshal(update.Params, &fullXLink); err != nil {
					c.logger.Error("failed to unmarshal update message", slog.Any("error", err))
					return
				}
				updateChan <- fullXLink
			case stats := <-statisticsChan:
				var rawStats xlink.StatsRaw
				if err := json.Unmarshal(stats.Params, &rawStats); err != nil {
					c.logger.Error("failed to unmarshal statistics message", slog.Any("error", err))
					return
				}
				statsChan <- rawStats
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			goto BREAK
		default:
			c.logger.Info("Connecting to xlink", slog.String("IP", c.ip))
			if err := c.connect(ctx, responseChan, statisticsChan); err != nil {
				c.logger.Error("failed connecting to xlink", slog.Any("error", err))
				time.Sleep(c.reconnectDelay)
			} else {
				goto BREAK
			}
		}
	}

BREAK:
	wg.Wait()
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
	err := c.jrpc.Listen(ctx, endpoint)

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

func (c *Client) onDisconnect() {
	c.authKey = ""
	c.ready.Store(false)
}
