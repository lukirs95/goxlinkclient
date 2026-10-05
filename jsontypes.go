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
//
// A value that cannot be interpreted at all never fails the surrounding
// message. The previous value is kept and the raw input is recorded in the
// Invalid field, so the caller can report it as a decodeIssue.

var jsonNull = []byte("null")

// decodeIssue describes a value that could not be decoded and was ignored.
type decodeIssue struct {
	Key string
	Raw json.RawMessage
}

func (i decodeIssue) String() string {
	return fmt.Sprintf("%s: %s", i.Key, i.Raw)
}

// unquote returns the trimmed content of a JSON string. ok is false if data is
// not a JSON string.
func unquote(data []byte) (s string, ok bool) {
	if len(data) == 0 || data[0] != '"' {
		return "", false
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return "", false
	}
	return strings.TrimSpace(s), true
}

// flexInt is an integer that the device encodes either as a JSON number or as
// a string holding a number, e.g. ports, RTP payload IDs or PTP settings.
//
// An empty string decodes to zero, because the web UI clears numeric fields by
// sending "". A JSON null leaves the current value untouched, matching how
// encoding/json treats null for plain integers. This keeps the previous value
// when a delta update is decoded into an existing struct.
type flexInt struct {
	Value int
	// Invalid holds the raw input if it could not be decoded. Value is left
	// unchanged in that case.
	Invalid json.RawMessage
}

// UnmarshalJSON implements json.Unmarshaler. It never returns an error.
func (i *flexInt) UnmarshalJSON(data []byte) error {
	i.Invalid = nil
	if bytes.Equal(data, jsonNull) {
		return nil
	}

	text := string(data)
	if s, ok := unquote(data); ok {
		if s == "" {
			i.Value = 0
			return nil
		}
		text = s
	}

	n, err := strconv.Atoi(text)
	if err != nil {
		i.Invalid = slices.Clone(data)
		return nil
	}
	i.Value = n
	return nil
}

// issue returns a decodeIssue for key if the last decoded input was invalid.
func (i flexInt) issue(key string) (decodeIssue, bool) {
	return decodeIssue{Key: key, Raw: i.Invalid}, i.Invalid != nil
}

// flexBool is a boolean that the device encodes either as a JSON boolean or as
// a string such as "true" or "false" (seen for sdilevelA). Any value accepted
// by strconv.ParseBool is valid. A JSON null leaves the current value
// untouched.
type flexBool struct {
	Value bool
	// Invalid holds the raw input if it could not be decoded. Value is left
	// unchanged in that case.
	Invalid json.RawMessage
}

// UnmarshalJSON implements json.Unmarshaler. It never returns an error.
func (b *flexBool) UnmarshalJSON(data []byte) error {
	b.Invalid = nil
	if bytes.Equal(data, jsonNull) {
		return nil
	}

	text := string(data)
	if s, ok := unquote(data); ok {
		text = s
	}

	v, err := strconv.ParseBool(text)
	if err != nil {
		b.Invalid = slices.Clone(data)
		return nil
	}
	b.Value = v
	return nil
}

// issue returns a decodeIssue for key if the last decoded input was invalid.
func (b flexBool) issue(key string) (decodeIssue, bool) {
	return decodeIssue{Key: key, Raw: b.Invalid}, b.Invalid != nil
}

// errorData is the "data" member of a JSON-RPC error sent by the device.
//
// Rejected config requests report the offending keys as an object, e.g.
// {"vModeLock":"Video Mode auto Not supported for Card 12"}. Failed actions
// such as resetSSRC report a plain string, e.g. "video not running". Any other
// JSON value is kept verbatim in Message so no information is lost.
type errorData struct {
	// Message holds the detail if the device sent anything but an object.
	Message string
	// Fields maps each rejected key to the reason if the device sent an object.
	Fields map[string]string
}

// UnmarshalJSON implements json.Unmarshaler. It never returns an error.
func (e *errorData) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		return nil
	}
	if s, ok := unquote(data); ok {
		e.Message = s
		return nil
	}
	if data[0] == '{' {
		var fields map[string]string
		if err := json.Unmarshal(data, &fields); err == nil {
			e.Fields = fields
			return nil
		}
	}
	e.Message = string(data)
	return nil
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
