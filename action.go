package xlinkclient

import (
	"context"
	"encoding/json"
	"fmt"
)

type actionMessage struct {
	System    string          `json:"sysid"`
	EnDecoder string `json:"id"`
}

type actionResponse struct {
	Response bool `json:"response"`
}

// StartEnDecoder lets you start any encoder or decoder by providing its identifier
func (c *client) StartEnDecoder(ctx context.Context, endecoder string) error {
	if res, err := c.jrpc.SendRequest(ctx, "start", actionMessage{c.systemId, endecoder}); err != nil {
		return err
	} else {
		var result actionResponse
		if err := json.Unmarshal(res, &result); err != nil {
			return err
		} else if !result.Response {
			return fmt.Errorf("could not start `%s`", endecoder)
		} else {
			return nil
		}
	}
}

// StopEnDecoder lets you start any encoder or decoder by providing its identifier
func (c *client) StopEnDecoder(ctx context.Context, endecoder string) error {
	if res, err := c.jrpc.SendRequest(ctx, "stop", actionMessage{
		System:    c.systemId,
		EnDecoder: endecoder,
	}); err != nil {
		return err
	} else {
		var result actionResponse
		if err := json.Unmarshal(res, &result); err != nil {
			return err
		} else if !result.Response {
			return fmt.Errorf("could not stop `%s`", endecoder)
		} else {
			return nil
		}
	}
}