package xlinkclient

import (
	"context"
	"encoding/json"
	"fmt"
)

type configureMessage struct {
	System    string          `json:"sysid"`
	EnDecoder string          `json:"id"`
	Values    configureValues `json:"values"`
}

type configureValues struct {
	VEnabled bool `json:"v2110NetPriEnabled"`
}

type configureResponse struct {
	Response bool `json:"response"`
}

// EnableVideo lets you enable Video transmission on encoders decoders
func (c *Client) EnableVideo(ctx context.Context, endecoder string) error {
	if res, err := c.jrpc.SendRequest(ctx, "config", configureMessage{
		System:    c.systemId,
		EnDecoder: endecoder,
		Values: configureValues{
			VEnabled: true,
		},
	}); err != nil {
		return err
	} else {
		var result configureResponse
		if err := json.Unmarshal(res, &result); err != nil {
			return err
		} else if !result.Response {
			return fmt.Errorf("could not enable video in `%s`", endecoder)
		} else {
			return nil
		}
	}
}

// EnableVideo lets you disable Video transmission on encoders decoders
func (c *Client) DisableVideo(ctx context.Context, endecoder string) error {
	if res, err := c.jrpc.SendRequest(ctx, "config", configureMessage{
		System:    c.systemId,
		EnDecoder: endecoder,
		Values: configureValues{
			VEnabled: false,
		},
	}); err != nil {
		return err
	} else {
		var result configureResponse
		if err := json.Unmarshal(res, &result); err != nil {
			return err
		} else if !result.Response {
			return fmt.Errorf("could not disable video in `%s`", endecoder)
		} else {
			return nil
		}
	}
}
