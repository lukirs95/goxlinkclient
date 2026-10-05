package xlinkclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The device firmware does not encode values consistently. Depending on the
// message and on how a value was last written, the same key may arrive as a
// JSON number or as a string holding that number, booleans may arrive as
// strings, and error details may be an object or a plain string. The types in
// this file absorb these differences while decoding. They are unexported on
// purpose: the public model only exposes plain Go types.

var jsonNull = []byte("null")

// flexInt is an integer that the device encodes either as a JSON number or as
// a string holding a number, e.g. ports, RTP payload IDs or PTP settings.
//
// An empty string decodes to zero, because the web UI clears numeric fields by
// sending "". A JSON null leaves the current value untouched, matching how
// encoding/json treats null for plain integers. This keeps the previous value
// when a delta update is decoded into an existing struct.
type flexInt int

// UnmarshalJSON implements json.Unmarshaler.
func (i *flexInt) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}

	text := string(data)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		text = strings.TrimSpace(s)
		if text == "" {
			*i = 0
			return nil
		}
	}

	n, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("xlinkclient: cannot decode %s as integer: %w", data, err)
	}
	*i = flexInt(n)
	return nil
}

// flexBool is a boolean that the device encodes either as a JSON boolean or as
// a string such as "true" or "false" (seen for sdilevelA). Any value accepted
// by strconv.ParseBool is valid. A JSON null leaves the current value
// untouched.
type flexBool bool

// UnmarshalJSON implements json.Unmarshaler.
func (b *flexBool) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}

	text := string(data)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		text = strings.TrimSpace(s)
	}

	v, err := strconv.ParseBool(text)
	if err != nil {
		return fmt.Errorf("xlinkclient: cannot decode %s as boolean: %w", data, err)
	}
	*b = flexBool(v)
	return nil
}

// errorData is the "data" member of a JSON-RPC error sent by the device.
//
// Rejected config requests report the offending keys as an object, e.g.
// {"vModeLock":"Video Mode auto Not supported for Card 12"}. Failed actions
// such as resetSSRC report a plain string, e.g. "video not running".
type errorData struct {
	// Message holds the detail if the device sent a plain string.
	Message string
	// Fields maps each rejected key to the reason if the device sent an object.
	Fields map[string]string
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *errorData) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}

	switch data[0] {
	case '"':
		return json.Unmarshal(data, &e.Message)
	case '{':
		return json.Unmarshal(data, &e.Fields)
	default:
		return fmt.Errorf("xlinkclient: unexpected error data %s", data)
	}
}

// String returns the detail in a human readable form. Fields are sorted by key
// so the result is stable.
func (e errorData) String() string {
	if len(e.Fields) == 0 {
		return e.Message
	}

	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+e.Fields[k])
	}
	return strings.Join(parts, "; ")
}
