package xlinkclient

import (
	"context"
	"encoding/json"

	jsonrpc "github.com/lukirs95/gojsonrpc"
)

const (
	method_auth jsonrpc.Method = "auth"
)

var authMessage struct {
	Auth     bool   `json:"auth"`
	UserId   string `json:"userid"`
	Password string `json:"pass"`
} = struct {
	Auth     bool   `json:"auth"`
	UserId   string `json:"userid"`
	Password string `json:"pass"`
}{Auth: true, UserId: "admin"}

type authResponse struct {
	AuthKey string `json:"authKey"`
}

type authAdvise struct {
	SystemId string `json:"sysid"`
}

func (c *Client) asyncAuthenticate(ctx context.Context, adviseChan jsonrpc.Subscription) {
	select {
	case <-ctx.Done():
		return
	case rawAdvise := <-adviseChan:
		advise := authAdvise{}
		if err := json.Unmarshal(rawAdvise.Params, &advise); err != nil {
			return
		}
		c.systemId = advise.SystemId

		response, err := c.jrpc.SendRequest(context.Background(), method_auth, authMessage)

		if err != nil {
			c.authKey = ""
			return
		}

		var authResponse authResponse
		err = json.Unmarshal(response, &authResponse)
		if err != nil {
			c.authKey = ""
		}
		c.authKey = authResponse.AuthKey
		c.ready.Store(true)
	}
}
