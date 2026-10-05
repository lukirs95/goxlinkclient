package xlinkclient

import (
	"encoding/json"
	"maps"
	"testing"
)

func TestFlexIntUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		initial flexInt
		want    flexInt
		wantErr bool
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
		{name: "float", input: `1.5`, wantErr: true},
		{name: "float string", input: `"1.5"`, wantErr: true},
		{name: "non numeric string", input: `"auto"`, wantErr: true},
		{name: "boolean", input: `true`, wantErr: true},
		{name: "object", input: `{}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.initial
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = %d, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Unmarshal(%s) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestFlexIntInStruct(t *testing.T) {
	type values struct {
		Port    flexInt `json:"v2110NetPriPort"`
		Payload flexInt `json:"v2110RTPpayload"`
	}

	// A delta that only carries the payload must not reset the port.
	got := values{Port: 30000, Payload: 96}
	if err := json.Unmarshal([]byte(`{"v2110RTPpayload":"97"}`), &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	want := values{Port: 30000, Payload: 97}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
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
	if p != (ports{From: 10502, To: 10539}) {
		t.Errorf("got %+v, want {From:10502 To:10539}", p)
	}
}

func TestFlexBoolUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		initial flexBool
		want    flexBool
		wantErr bool
	}{
		{name: "true", input: `true`, want: true},
		{name: "false", input: `false`, initial: true, want: false},
		// The web UI sends sdilevelA as a string.
		{name: "string true", input: `"true"`, want: true},
		{name: "string false", input: `"false"`, initial: true, want: false},
		{name: "null keeps value", input: `null`, initial: true, want: true},
		{name: "empty string", input: `""`, wantErr: true},
		{name: "unknown string", input: `"yes"`, wantErr: true},
		{name: "object", input: `{}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.initial
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = %t, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Unmarshal(%s) = %t, want %t", tt.input, got, tt.want)
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
		wantErr    bool
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
		{name: "number", input: `1`, wantErr: true},
		{name: "array", input: `["x"]`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got errorData
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = %+v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
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
