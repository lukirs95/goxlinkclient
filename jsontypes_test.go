package xlinkclient

import (
	"encoding/json"
	"maps"
	"testing"
)

func TestFlexIntUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		initial     int
		want        int
		wantInvalid bool
	}{
		// state.subscribe reports 2110 ports as numbers ...
		{name: "number", input: `30000`, want: 30000},
		// ... while systems.full reports them as strings.
		{name: "numeric string", input: `"5000"`, want: 5000},
		{name: "zero string", input: `"0"`, initial: 7, want: 0},
		// PTP intervals are negative and written as strings by the web UI.
		{name: "negative number", input: `-3`, want: -3},
		{name: "negative string", input: `"-2"`, want: -2},
		{name: "string with surrounding spaces", input: `" 10501 "`, want: 10501},
		// The web UI clears manPort by sending "".
		{name: "empty string", input: `""`, initial: 10501, want: 0},
		{name: "blank string", input: `"  "`, initial: 10501, want: 0},
		// aSync was null in firmware 1.7 and a number in 1.8.
		{name: "null keeps value", input: `null`, initial: 42, want: 42},
		// systems.full reports the encoder bitrate (vTBR) as 25.0.
		{name: "integral float", input: `25.0`, want: 25},
		{name: "integral float string", input: `"5.0"`, want: 5},
		{name: "integral float exponent", input: `1e3`, want: 1000},
		// Invalid input keeps the previous value and is reported.
		{name: "float", input: `1.5`, initial: 9, want: 9, wantInvalid: true},
		{name: "float string", input: `"1.5"`, initial: 9, want: 9, wantInvalid: true},
		{name: "non numeric string", input: `"auto"`, initial: 9, want: 9, wantInvalid: true},
		{name: "boolean", input: `true`, initial: 9, want: 9, wantInvalid: true},
		{name: "object", input: `{}`, initial: 9, want: 9, wantInvalid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flexInt{Value: tt.initial}
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.input, err)
			}
			if got.Value != tt.want {
				t.Errorf("Unmarshal(%s) = %d, want %d", tt.input, got.Value, tt.want)
			}
			issue, invalid := got.issue("key")
			if invalid != tt.wantInvalid {
				t.Fatalf("issue() reported %t, want %t", invalid, tt.wantInvalid)
			}
			if invalid && (issue.Key != "key" || string(issue.Raw) != tt.input) {
				t.Errorf("issue() = %v, want key: %s", issue, tt.input)
			}
		})
	}
}

func TestFlexIntResetsInvalid(t *testing.T) {
	var got flexInt
	if err := json.Unmarshal([]byte(`"auto"`), &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if err := json.Unmarshal([]byte(`5000`), &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if _, invalid := got.issue("key"); invalid {
		t.Error("issue() still reported after a valid value was decoded")
	}
	if got.Value != 5000 {
		t.Errorf("Value = %d, want 5000", got.Value)
	}
}

func TestFlexIntInStruct(t *testing.T) {
	type values struct {
		Port    flexInt `json:"v2110NetPriPort"`
		Payload flexInt `json:"v2110RTPpayload"`
	}

	// A delta that only carries the payload must not reset the port.
	got := values{Port: flexInt{Value: 30000}, Payload: flexInt{Value: 96}}
	if err := json.Unmarshal([]byte(`{"v2110RTPpayload":"97"}`), &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if got.Port.Value != 30000 || got.Payload.Value != 97 {
		t.Errorf("got port %d payload %d, want 30000 and 97", got.Port.Value, got.Payload.Value)
	}

	// An invalid value must not stop the remaining fields from being decoded.
	if err := json.Unmarshal([]byte(`{"v2110NetPriPort":"auto","v2110RTPpayload":98}`), &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if got.Port.Value != 30000 || got.Payload.Value != 98 {
		t.Errorf("got port %d payload %d, want 30000 and 98", got.Port.Value, got.Payload.Value)
	}
	if _, invalid := got.Port.issue("v2110NetPriPort"); !invalid {
		t.Error("invalid port was not reported")
	}

	// Mixed representations within one message, as sent by configSysPorts.
	type ports struct {
		From flexInt `json:"portsFrom"`
		To   flexInt `json:"portsTo"`
	}
	var p ports
	if err := json.Unmarshal([]byte(`{"portsFrom":"10502","portsTo":10539}`), &p); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if p.From.Value != 10502 || p.To.Value != 10539 {
		t.Errorf("got %d-%d, want 10502-10539", p.From.Value, p.To.Value)
	}
}

func TestFlexBoolUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		initial     bool
		want        bool
		wantInvalid bool
	}{
		{name: "true", input: `true`, want: true},
		{name: "false", input: `false`, initial: true, want: false},
		// The web UI sends sdilevelA as a string.
		{name: "string true", input: `"true"`, want: true},
		{name: "string false", input: `"false"`, initial: true, want: false},
		{name: "null keeps value", input: `null`, initial: true, want: true},
		// Invalid input keeps the previous value and is reported.
		{name: "empty string", input: `""`, initial: true, want: true, wantInvalid: true},
		{name: "unknown string", input: `"yes"`, initial: true, want: true, wantInvalid: true},
		{name: "object", input: `{}`, initial: true, want: true, wantInvalid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flexBool{Value: tt.initial}
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.input, err)
			}
			if got.Value != tt.want {
				t.Errorf("Unmarshal(%s) = %t, want %t", tt.input, got.Value, tt.want)
			}
			if _, invalid := got.issue("key"); invalid != tt.wantInvalid {
				t.Errorf("issue() reported %t, want %t", invalid, tt.wantInvalid)
			}
		})
	}
}

func TestErrorDataUnmarshal(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMsg    string
		wantFields map[string]string
		wantString string
	}{
		{
			name:       "action error is a string",
			input:      `"video not running"`,
			wantMsg:    "video not running",
			wantString: "video not running",
		},
		{
			name:       "config error is an object",
			input:      `{"vModeLock":"Video Mode auto Not supported for Card 12"}`,
			wantFields: map[string]string{"vModeLock": "Video Mode auto Not supported for Card 12"},
			wantString: "vModeLock: Video Mode auto Not supported for Card 12",
		},
		{
			name:       "fields are sorted in String",
			input:      `{"b":"second","a":"first"}`,
			wantFields: map[string]string{"a": "first", "b": "second"},
			wantString: "a: first; b: second",
		},
		{name: "null", input: `null`},
		// Unexpected shapes are kept verbatim instead of being dropped.
		{name: "number", input: `1`, wantMsg: "1", wantString: "1"},
		{name: "array", input: `["x"]`, wantMsg: `["x"]`, wantString: `["x"]`},
		{name: "object with non string values", input: `{"a":1}`, wantMsg: `{"a":1}`, wantString: `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got errorData
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.input, err)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
			if !maps.Equal(got.Fields, tt.wantFields) {
				t.Errorf("Fields = %v, want %v", got.Fields, tt.wantFields)
			}
			if s := got.String(); s != tt.wantString {
				t.Errorf("String() = %q, want %q", s, tt.wantString)
			}
		})
	}
}

// TestErrorDataInResponse decodes a complete error response as captured from
// the device to make sure errorData works as a nested field.
func TestErrorDataInResponse(t *testing.T) {
	const response = `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Internal error",` +
		`"data":{"vModeLock":"Video Mode auto Not supported for Card 12"}},"id":266}`

	var msg struct {
		Error struct {
			Code    int       `json:"code"`
			Message string    `json:"message"`
			Data    errorData `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(response), &msg); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if msg.Error.Code != -32603 {
		t.Errorf("Code = %d, want -32603", msg.Error.Code)
	}
	if got := msg.Error.Data.Fields["vModeLock"]; got != "Video Mode auto Not supported for Card 12" {
		t.Errorf("Fields[vModeLock] = %q", got)
	}
}
