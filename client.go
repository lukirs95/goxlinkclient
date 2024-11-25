package xlinkclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	jsonrpc "github.com/lukirs95/gojsonrpc"
	xlink "github.com/lukirs95/goxlink/internal/model"
)

type clientOption func(c *client)

// WithLogger is an option you can provide to you use your own logger.
func WithLogger(logger Logger) clientOption {
	return func(c *client) {
		c.logger = logger
	}
}

// WithLogger lets you adjust the automatic reconnection if an error accurs.
func WithReconnect(interval time.Duration) clientOption {
	return func(c *client) {
		c.reconnectDelay = interval
	}
}

// WithPassword lets you adjust the password that is been used to connect to the system
func WithPassword(password string) clientOption {
	return func(c *client) {
		c.password = password
	}
}

type client struct {
	logger Logger
	reconnectDelay time.Duration
	ip string
	password string
	jrpc     *jsonrpc.JsonRPC
	authKey  string
	systemId string
	ready    atomic.Bool
}

// NewClient creates a new instance of the client. The client handles the connection.
func NewClient(ip string, opts ...clientOption) *client {
	c := &client{
		jrpc:    jsonrpc.NewJsonRPC(),
		authKey: "",
		ready:   atomic.Bool{},
		ip: ip,
	}

	for _, option := range opts {
		option(c)
	}

	if c.password == "" {
		c.password = "123456!"
	}

	if c.logger == nil {
		c.logger = DefaultLogger{}
	}

	if c.reconnectDelay == 0 {
		c.reconnectDelay = time.Second * 10
	}

	authMessage.Password = c.password

	c.jrpc.OnDisconnect = c.onDisconnect
	return c
}

type UpdateChan chan xlink.XLink
type StatsChan chan xlink.Stats

// Connect attempts to connect to the system. It is blocking!
// If you cancle the context, the connection is closed.
func (c *client) Connect(ctx context.Context, updateChan UpdateChan, statsChan StatsChan) {
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
					c.logger.Error(err)
					return
				}
				updateChan<- fullXLink
			case stats := <-statisticsChan:
				var rawStats xlink.StatsRaw
				if err := json.Unmarshal(stats.Params, &rawStats); err != nil {
					c.logger.Error(err)
					return
				}
				statsChan<-rawStats
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			goto BREAK
		default:
			if err := c.connect(ctx, responseChan, statisticsChan); err != nil {
				c.logger.Error(err)
				c.logger.Infof("try reconnect to %s", c.ip)
				time.Sleep(c.reconnectDelay)
			} else {
				goto BREAK
			}
		}
	}

	BREAK:
	wg.Wait()
}

func (c *client) connect(ctx context.Context, responseChan jsonrpc.Subscription, statsChan jsonrpc.Subscription) error {
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

// Ready returns true if the client is connected to the system.
func (c *client) Ready() bool {
	return c.ready.Load()
}

func (c *client) onDisconnect() {
	c.authKey = ""
	c.ready.Store(false)
}
