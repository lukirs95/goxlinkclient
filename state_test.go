package xlinkclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

// applyUpdates applies a systems.update message and returns its updates.
func applyUpdates(t *testing.T, st *state, params json.RawMessage) []Update {
	t.Helper()
	var c collector
	updates, _, err := st.applyUpdate(&c, params)
	if err != nil {
		t.Fatalf("applyUpdate: %v", err)
	}
	for _, issue := range c.issues {
		t.Errorf("unexpected decode issue: %v", issue)
	}
	return updates
}

// describe lists updates as "Kind ID" or "Kind ID removed", leaving out
// peers, whose last seen time changes with nearly every message.
func describe(updates []Update) []string {
	var out []string
	for _, u := range updates {
		if u.Kind == UpdatePeer {
			continue
		}
		s := fmt.Sprintf("%v %s", u.Kind, u.ID)
		if u.Removed {
			s += " removed"
		}
		out = append(out, s)
	}
	return out
}

func readUpdates(t *testing.T, name string) []json.RawMessage {
	t.Helper()
	var updates []json.RawMessage
	if err := json.Unmarshal(readFixture(t, name), &updates); err != nil {
		t.Fatal(err)
	}
	return updates
}

func TestUpdatesHeartbeat(t *testing.T) {
	st := loadState(t, "systems_full_1.8.json")
	// The heartbeat touches the start times, all link times and the last
	// seen time of a peer. Only the link time of eth3 and the peer changed.
	updates := applyUpdates(t, st, readFixture(t, "systems_update_heartbeat_1.8.json"))
	if got := describe(updates); !slices.Equal(got, []string{"Interface eth3"}) {
		t.Errorf("updates = %v, want [Interface eth3]", got)
	}
	// The update carries the complete interface.
	if i := updates[0].Interface; i == nil || i.ID != "eth3" || i.IP == "" && i.MAC == "" {
		t.Errorf("interface = %+v", i)
	}
	// The same heartbeat again changes nothing.
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(readFixture(t, "systems_update_heartbeat_1.8.json"), &msg); err != nil {
		t.Fatal(err)
	}
	msg["dataid"] = json.RawMessage("776")
	again, _ := json.Marshal(msg)
	if u := applyUpdates(t, st, again); len(u) != 0 {
		t.Errorf("repeated heartbeat: updates = %v", describe(u))
	}
}

func TestUpdatesAddChangeDelete(t *testing.T) {
	st := loadState(t, "systems_full_1.8.json")
	want := [][]string{
		// eth3 came up again; the other link times only jitter.
		{"Encoder X8A1001-E6", "Interface eth3"},
		{"Decoder X8A1001-D6"},
		{"Decoder X8A1001-D6"},
		{"Decoder X8A1001-D6 removed"},
		{"Encoder X8A1001-E6 removed"},
	}
	for i, msg := range readUpdates(t, "systems_update_add_delete_1.8.json") {
		updates := applyUpdates(t, st, msg)
		if got := describe(updates); !slices.Equal(got, want[i]) {
			t.Errorf("message %d: updates = %v, want %v", i, got, want[i])
		}
		for _, u := range updates {
			switch {
			case u.Kind == UpdateEncoder && !u.Removed && (u.Encoder == nil || u.Encoder.Name == ""):
				t.Errorf("message %d: incomplete encoder %+v", i, u.Encoder)
			case u.Kind == UpdateDecoder && !u.Removed && (u.Decoder == nil || u.Decoder.Name == ""):
				t.Errorf("message %d: incomplete decoder %+v", i, u.Decoder)
			case u.Removed && (u.Encoder != nil || u.Decoder != nil):
				t.Errorf("message %d: removal carries an element", i)
			}
		}
	}
}

func TestUpdatesSRTAndNDI(t *testing.T) {
	st := loadState(t, "systems_full_1.8.json")
	want := [][]string{
		{"NDIEncoder X8A1001-NdiE6", "Interface eth3"},
		{"NDIDecoder X8A1001-NdiD6"},
		{"SRTEncoder X8A1001-srtE1"},
		{"SRTDecoder X8A1001-srtD1"},
		{"SRTEncoder X8A1001-srtE1 removed"},
		{"SRTDecoder X8A1001-srtD1 removed"},
		{"NDIEncoder X8A1001-NdiE6 removed"},
		{"NDIDecoder X8A1001-NdiD6 removed"},
	}
	for i, msg := range readUpdates(t, "systems_update_srt_ndi_1.8.json") {
		if got := describe(applyUpdates(t, st, msg)); !slices.Equal(got, want[i]) {
			t.Errorf("message %d: updates = %v, want %v", i, got, want[i])
		}
	}
}

func TestUpdatesCompleteElements(t *testing.T) {
	st := loadState(t, "systems_full_1.8.json")

	// A value that does not change produces no update, a changed one the
	// complete encoder.
	if u := applyUpdates(t, st, json.RawMessage(`{"dataid":770,"data":{"local":{"enc":[{"id":"X8A1001-E1","values":{"vTBR":25.0}}]}}}`)); len(u) != 0 {
		t.Errorf("unchanged bitrate: updates = %v", describe(u))
	}
	before, _ := mustSnapshot(t, st).Encoder("X8A1001-E1")
	updates := applyUpdates(t, st, json.RawMessage(`{"dataid":771,"data":{"local":{"name":"NEW","enc":[{"id":"X8A1001-E1","values":{"vTBR":12}}]}}}`))
	if got := describe(updates); !slices.Equal(got, []string{"System ", "Encoder X8A1001-E1"}) {
		t.Fatalf("updates = %v", got)
	}
	if updates[0].Info.Name != "NEW" || updates[0].Info.ID != "X8A1001" {
		t.Errorf("system info = %+v", updates[0].Info)
	}
	want := before
	want.Bitrate = 12
	if got := *updates[1].Encoder; !got.equal(&want) {
		t.Errorf("encoder = %+v\nwant      %+v", got, want)
	}

	// A peer is reported with its units and interfaces.
	updates = applyUpdates(t, st, json.RawMessage(`{"dataid":772,"data":{"remote":[{"sysid":"X8A1008","lastSeenT":"2026-10-05 12:00:00 +0000"}]}}`))
	if len(updates) != 1 || updates[0].Peer == nil || len(updates[0].Peer.Encoders) != 5 {
		t.Errorf("peer updates = %+v", updates)
	}
}

func TestUpdatesUnitConfig(t *testing.T) {
	st := loadState(t, "systems_full_1.8.json")
	st.configs = configs{}
	for _, id := range []UnitID{"X8A1001-E1", "X4A1004-D1"} {
		st.configs.expect(id)
	}
	apply := func(raw string) []Update {
		t.Helper()
		var c collector
		updates, _, err := st.applyUnitUpdate(&c, json.RawMessage(raw))
		if err != nil {
			t.Fatalf("applyUnitUpdate: %v", err)
		}
		return updates
	}

	// The response of the encoder changes the encoder.
	var responses []json.RawMessage
	if err := json.Unmarshal(readFixture(t, "state_subscribe_1.8.json"), &responses); err != nil {
		t.Fatal(err)
	}
	var c collector
	updates, err := st.applyUnitSubscribe(&c, responses[0])
	if err != nil || len(updates) != 1 || updates[0].Encoder.Config == nil || updates[0].Encoder.Config.Bitrate != 18 {
		t.Fatalf("subscribe: err %v updates %+v", err, updates)
	}

	// A change of the configuration delivers the complete encoder.
	updates = apply(`{"id":"X8A1001-E1","dataid":2,"data":{"values":{"vTBR":25}}}`)
	if len(updates) != 1 || updates[0].Kind != UpdateEncoder || updates[0].Encoder.Config.Bitrate != 25 ||
		updates[0].Encoder.Config.Name != "Encoder 106" || updates[0].Encoder.Receiver.ID != "X4A1004-D1" {
		t.Errorf("config update = %+v", updates)
	}
	// Unchanged values and updates without values produce nothing.
	if u := apply(`{"id":"X8A1001-E1","dataid":3,"data":{"values":{"vTBR":25}}}`); len(u) != 0 {
		t.Errorf("unchanged config: %v", describe(u))
	}
	if u := apply(`{"id":"X8A1001-E1","dataid":4,"data":{"xlinkRTT":0.3}}`); len(u) != 0 {
		t.Errorf("update without values: %v", describe(u))
	}

	// The configuration of a counterpart changes the local unit linked to it.
	response := `{"method":"state.subscribe","id":"X4A1004-D1","dataid":1,"response":true,"data":{"values":{"name":"DEC-7","pbuf":300}}}`
	updates, err = st.applyUnitSubscribe(&c, json.RawMessage(response))
	if err != nil || len(updates) != 1 || updates[0].ID != "X8A1001-E1" ||
		updates[0].Encoder.Receiver.Config == nil || updates[0].Encoder.Receiver.Config.PacketBuffer.Milliseconds() != 300 {
		t.Errorf("counterpart subscribe: err %v updates %+v", err, updates)
	}
}

// BenchmarkApplyHeartbeat measures the most frequent message: a heartbeat
// that touches link and start times but changes nothing.
func BenchmarkApplyHeartbeat(b *testing.B) {
	full, err := os.ReadFile(filepath.Join("testdata", "systems_full_1.8.json"))
	if err != nil {
		b.Fatal(err)
	}
	heartbeat, err := os.ReadFile(filepath.Join("testdata", "systems_update_heartbeat_1.8.json"))
	if err != nil {
		b.Fatal(err)
	}
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(heartbeat, &msg); err != nil {
		b.Fatal(err)
	}
	messages := make([][]byte, b.N)
	for i := range messages {
		msg["dataid"] = json.RawMessage(strconv.Itoa(1000 + i))
		messages[i], _ = json.Marshal(msg)
	}
	var st state
	if err := st.applyFull(full); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for _, m := range messages {
		var c collector
		if _, _, err := st.applyUpdate(&c, m); err != nil {
			b.Fatal(err)
		}
	}
}
