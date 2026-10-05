package xlinkclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	jsonrpc "github.com/lukirs95/gojsonrpc/v2"
)

// authTimeout is how long to wait for the device to announce itself after the
// connection was opened.
const authTimeout = 5 * time.Second

var (
	// ErrAuthFailed is returned by Run if the device rejected the credentials.
	ErrAuthFailed = errors.New("xlinkclient: authentication failed")
	// ErrAuthTimeout is returned by Run if the device did not ask for
	// authentication in time.
	ErrAuthTimeout = errors.New("xlinkclient: device did not request authentication")
)

type authParams struct {
	Auth     bool   `json:"auth"`
	UserID   string `json:"userid"`
	Password string `json:"pass"`
}

// authenticate waits for the device's notify.auth message, which carries the
// system ID, and logs in. It returns nil if ctx is done first.
func (c *Client) authenticate(ctx context.Context, advice jsonrpc.Subscription) error {
	timer := time.NewTimer(authTimeout)
	defer timer.Stop()

	var msg jsonrpc.Notification
	select {
	case <-ctx.Done():
		return nil
	case <-timer.C:
		return ErrAuthTimeout
	case msg = <-advice:
	}

	var adv struct {
		SysID SystemID `json:"sysid"`
	}
	if err := json.Unmarshal(msg.Params, &adv); err != nil {
		return fmt.Errorf("xlinkclient: decode %s: %w", notifyAuthentication, err)
	}
	c.mu.Lock()
	c.systemID = adv.SysID
	c.mu.Unlock()

	raw, err := c.jrpc.SendRequest(ctx, methodAuth, authParams{Auth: true, UserID: c.user, Password: c.password})
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		return fmt.Errorf("%w: %w", ErrAuthFailed, deviceError(methodAuth, rpcErr))
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("xlinkclient: authenticate: %w", err)
	}
	var res result
	if err := json.Unmarshal(raw, &res); err != nil {
		return fmt.Errorf("xlinkclient: decode auth response: %w", err)
	}
	if !res.Response {
		return ErrAuthFailed
	}

	c.setReady(true)
	c.logger.Info("authenticated", slog.String("sysid", string(adv.SysID)))
	return nil
}
