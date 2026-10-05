package xlinkclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	jsonrpc "github.com/lukirs95/gojsonrpc/v2"
)

// JSON-RPC methods sent by the client.
const (
	methodAuth           jsonrpc.Method = "auth"
	methodConfig         jsonrpc.Method = "config"
	methodStart          jsonrpc.Method = "start"
	methodStop           jsonrpc.Method = "stop"
	methodResetStats     jsonrpc.Method = "resetVstat"
	methodResetBuffer    jsonrpc.Method = "resetSSRC"
	methodFlushAudio     jsonrpc.Method = "flushAudio"
	methodNewVideo       jsonrpc.Method = "newVideo"
	methodDeleteVideo    jsonrpc.Method = "deleteVideo"
	methodConfigEth      jsonrpc.Method = "configEth"
	methodSet2110        jsonrpc.Method = "set2110"
	methodDNS            jsonrpc.Method = "dnsSys"
	methodSystemMTU      jsonrpc.Method = "setSystemMTU"
	methodSystemName     jsonrpc.Method = "configSysName"
	methodSystemPorts    jsonrpc.Method = "configSysPorts"
	methodManualIP       jsonrpc.Method = "manIpPeer"
	methodAddPeer        jsonrpc.Method = "manAddPeer"
	methodNewTrunk       jsonrpc.Method = "newL2S"
	methodConfigTrunk    jsonrpc.Method = "configL2S"
	methodStartTrunk     jsonrpc.Method = "startL2S"
	methodStopTrunk      jsonrpc.Method = "stopL2S"
	methodDeleteTrunk    jsonrpc.Method = "deleteL2S"
	methodLocalStats     jsonrpc.Method = "localStats.subscribe"
	notifySystemsFull    jsonrpc.Method = "systems.full"
	notifySystemsUpdate  jsonrpc.Method = "systems.update"
	notifySystemsStats   jsonrpc.Method = "systems.stats"
	notifyLocalStats     jsonrpc.Method = "systems.localStats"
	notifyAuthentication jsonrpc.Method = "notify.auth"
)

// ErrNotConnected is returned by requests while the client is not connected
// and authenticated.
var ErrNotConnected = errors.New("xlinkclient: not connected")

// DeviceError is returned when the device rejects a request, either with a
// JSON-RPC error or by answering "response": false.
type DeviceError struct {
	// Method is the JSON-RPC method of the rejected request.
	Method string
	// Code and Message are the JSON-RPC error, e.g. -32603 "Internal error".
	// They are zero if the device answered "response": false.
	Code    int
	Message string
	// Detail is the device's explanation of a failed action, e.g.
	// "video not running".
	Detail string
	// Fields maps each rejected key of a config request to the reason, e.g.
	// "vModeLock": "Video Mode auto Not supported for Card 12".
	Fields map[string]string
}

func (e *DeviceError) Error() string {
	msg := "xlinkclient: device rejected " + e.Method
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if detail := (errorData{Message: e.Detail, Fields: e.Fields}).String(); detail != "" {
		msg += " (" + detail + ")"
	}
	return msg
}

// deviceError converts a JSON-RPC error of the device.
func deviceError(method jsonrpc.Method, rpcErr *jsonrpc.Error) *DeviceError {
	var data errorData
	if len(rpcErr.Data) > 0 {
		_ = data.UnmarshalJSON(rpcErr.Data)
	}
	return &DeviceError{
		Method:  string(method),
		Code:    int(rpcErr.Code),
		Message: rpcErr.Message,
		Detail:  data.Message,
		Fields:  data.Fields,
	}
}

// field is one key of a request's "values" object.
type field struct {
	key   string
	value any
}

// fields collects the keys of a request in the order they were set. The order
// matters when keys are sent one request at a time, e.g. a trunk's MTU switch
// must be set before its value.
type fields []field

func (f *fields) set(key string, value any) {
	*f = append(*f, field{key: key, value: value})
}

// object returns the fields as a JSON object. A key set more than once keeps
// its last value.
func (f fields) object() map[string]any {
	m := make(map[string]any, len(f))
	for _, kv := range f {
		m[kv.key] = kv.value
	}
	return m
}

// result is the common part of all responses.
type result struct {
	Response bool `json:"response"`
}

// call sends a request and checks the device's response flag.
func (c *Client) call(ctx context.Context, method jsonrpc.Method, params any) error {
	if !c.Ready() {
		return ErrNotConnected
	}
	raw, err := c.jrpc.SendRequest(ctx, method, params)
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		return deviceError(method, rpcErr)
	}
	if err != nil {
		return fmt.Errorf("xlinkclient: %s: %w", method, err)
	}
	var res result
	if err := json.Unmarshal(raw, &res); err != nil {
		return fmt.Errorf("xlinkclient: %s: decode response: %w", method, err)
	}
	if !res.Response {
		return &DeviceError{Method: string(method)}
	}
	return nil
}

// unitParams addresses a unit. sysid is always the local system, also for
// units of remote systems.
type unitParams struct {
	SysID  SystemID       `json:"sysid"`
	ID     string         `json:"id"`
	Values map[string]any `json:"values,omitempty"`
}

// systemParams addresses a system without a unit.
type systemParams struct {
	SysID  SystemID       `json:"sysid"`
	Values map[string]any `json:"values,omitempty"`
}

// callUnit sends an action or config request for a unit or trunk.
func (c *Client) callUnit(ctx context.Context, method jsonrpc.Method, sys SystemID, id string, values map[string]any) error {
	return c.call(ctx, method, unitParams{SysID: sys, ID: id, Values: values})
}

// callSystem sends a request with the given values for the local system.
func (c *Client) callSystem(ctx context.Context, method jsonrpc.Method, values map[string]any) error {
	return c.call(ctx, method, systemParams{SysID: c.SystemID(), Values: values})
}

// callSystemEach sends one request per field, in order, for methods where
// combining keys has not been verified on the device. It stops at the first
// error.
func (c *Client) callSystemEach(ctx context.Context, method jsonrpc.Method, f fields) error {
	for _, kv := range f {
		if err := c.callSystem(ctx, method, map[string]any{kv.key: kv.value}); err != nil {
			return fmt.Errorf("%w (key %s)", err, kv.key)
		}
	}
	return nil
}
