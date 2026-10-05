package xlinkclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// loadState applies the given systems.full fixture and fails on any issue.
func loadState(t *testing.T, name string) *state {
	t.Helper()
	var st state
	if err := st.applyFull(readFixture(t, name)); err != nil {
		t.Fatalf("applyFull(%s) returned error: %v", name, err)
	}
	return &st
}

func mustSnapshot(t *testing.T, st *state) System {
	t.Helper()
	sys, issues := st.snapshot()
	for _, issue := range issues {
		t.Errorf("unexpected decode issue: %v", issue)
	}
	return sys
}

func mustApplyUpdate(t *testing.T, st *state, params []byte) {
	t.Helper()
	gap, err := st.applyUpdate(params)
	if err != nil {
		t.Fatalf("applyUpdate returned error: %v", err)
	}
	if gap {
		t.Fatalf("applyUpdate reported a gap after dataid %d", st.wire.DataID.Value)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(deviceTimeLayout, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFullState18(t *testing.T) {
	sys := mustSnapshot(t, loadState(t, "systems_full_1.8.json"))

	if sys.ID != "X8A1001" || sys.Version != "1.8.4.6" || !sys.ST2110 {
		t.Errorf("system = %s %s st2110=%t, want X8A1001 1.8.4.6 st2110=true", sys.ID, sys.Version, sys.ST2110)
	}
	wantPorts := Ports{SystemStatic: true, SystemPort: 10501, DataStatic: true, DataFrom: 10502, DataTo: 10539}
	if sys.Ports != wantPorts {
		t.Errorf("Ports = %+v, want %+v", sys.Ports, wantPorts)
	}
	if sys.TrunkMTU != 1460 {
		t.Errorf("TrunkMTU = %d, want 1460", sys.TrunkMTU)
	}
	if len(sys.Encoders) != 5 || len(sys.Decoders) != 5 || len(sys.Interfaces) != 8 || len(sys.Peers) != 5 {
		t.Fatalf("counts enc=%d dec=%d nets=%d peers=%d, want 5/5/8/5",
			len(sys.Encoders), len(sys.Decoders), len(sys.Interfaces), len(sys.Peers))
	}

	enc, ok := sys.Encoder("X8A1001-E1")
	if !ok {
		t.Fatal("encoder X8A1001-E1 not found")
	}
	if enc.Card != VideoCardST2110 {
		t.Errorf("Card = %v, want 2110 (sent as string \"12\")", enc.Card)
	}
	// The unit itself reports the port as a number ...
	wantVideo := Stream{Interface: "eth6", Enabled: enc.Video2110.Enabled, Address: "239.0.0.3", Port: 30000}
	if enc.Video2110 != wantVideo {
		t.Errorf("Video2110 = %+v, want %+v", enc.Video2110, wantVideo)
	}
	if enc.Receiver.ID != "X4A1004-D1" {
		t.Errorf("Receiver.ID = %q, want X4A1004-D1", enc.Receiver.ID)
	}
	if enc.Receiver.Video != NoSignal || enc.Receiver.Video.Present() {
		t.Errorf("Receiver.Video = %q, want %q", enc.Receiver.Video, NoSignal)
	}
	if !enc.StartedAt.IsZero() {
		t.Errorf("StartedAt = %v, want zero time for the \"never\" placeholder", enc.StartedAt)
	}

	// An encoder without receiver reports "none".
	if e2, _ := sys.Encoder("X8A1001-E2"); e2.Receiver.ID != "" {
		t.Errorf("E2 Receiver.ID = %q, want empty for \"none\"", e2.Receiver.ID)
	}

	dec, ok := sys.Decoder("X8A1001-D1")
	if !ok {
		t.Fatal("decoder X8A1001-D1 not found")
	}
	if dec.SignalGenDelay != 60*time.Second || dec.Buffer != 1 || dec.AudioChannels != 0 {
		t.Errorf("decoder = delay %v buffer %d channels %d, want 60s/1/0", dec.SignalGenDelay, dec.Buffer, dec.AudioChannels)
	}
	if dec.Sender.ID != "" {
		t.Errorf("Sender.ID = %q, want empty for \"none\"", dec.Sender.ID)
	}

	eth6, ok := sys.Interface("eth6")
	if !ok {
		t.Fatal("interface eth6 not found")
	}
	if eth6.Speed != "25Gbps" || !eth6.LinkUp || !eth6.NMOS || eth6.Trunk != "" {
		t.Errorf("eth6 = %+v", eth6)
	}
	if want := mustTime(t, "2026-10-05 09:00:21 +0000"); !eth6.LinkSince.Equal(want) {
		t.Errorf("eth6 LinkSince = %v, want %v", eth6.LinkSince, want)
	}
	if eth7, _ := sys.Interface("eth7"); !eth7.LinkSince.IsZero() {
		t.Errorf("eth7 LinkSince = %v, want zero time", eth7.LinkSince)
	}

	peer, ok := sys.Peer("X8A1008")
	if !ok {
		t.Fatal("peer X8A1008 not found")
	}
	// conPort is a number for this peer and the string "0" for others.
	if peer.Port != 10501 || peer.ManualIP.Port != 10501 || len(peer.Encoders) != 5 {
		t.Errorf("peer = port %d manual %d encoders %d, want 10501/10501/5", peer.Port, peer.ManualIP.Port, len(peer.Encoders))
	}
	if peer.Encoders[0].Linked != "X4A1009-D1" {
		t.Errorf("peer encoder Linked = %q, want X4A1009-D1", peer.Encoders[0].Linked)
	}
	if other, _ := sys.Peer("X4A1007"); other.Port != 0 {
		t.Errorf("peer X4A1007 Port = %d, want 0 (sent as \"0\")", other.Port)
	}
}

func TestFullState17(t *testing.T) {
	sys := mustSnapshot(t, loadState(t, "systems_full_1.7.json"))

	if sys.Version != "1.7.2.16" {
		t.Errorf("Version = %q, want 1.7.2.16", sys.Version)
	}
	if len(sys.Encoders) != 2 || len(sys.Decoders) != 2 || len(sys.Peers) != 1 {
		t.Errorf("counts enc=%d dec=%d peers=%d, want 2/2/1", len(sys.Encoders), len(sys.Decoders), len(sys.Peers))
	}
}

func TestReceiverPortsAsStrings(t *testing.T) {
	// In systems.full the embedded receiver reports 2110 ports as strings while
	// the unit itself uses numbers. Both must decode without issues.
	st := loadState(t, "systems_full_1.8.json")
	var raw struct {
		Data struct {
			Local struct {
				Enc []struct {
					Receiver struct {
						Values struct {
							Port json.RawMessage `json:"v2110NetPriPort"`
						} `json:"values"`
					} `json:"receiver"`
				} `json:"enc"`
			} `json:"local"`
		} `json:"data"`
	}
	if err := json.Unmarshal(readFixture(t, "systems_full_1.8.json"), &raw); err != nil {
		t.Fatal(err)
	}
	if got := string(raw.Data.Local.Enc[0].Receiver.Values.Port); got != `"5000"` {
		t.Fatalf("fixture receiver port = %s, want the string \"5000\"", got)
	}
	mustSnapshot(t, st)
}

func TestUpdateAddAndDelete(t *testing.T) {
	st := loadState(t, "systems_full_1.8.json")

	var updates []json.RawMessage
	if err := json.Unmarshal(readFixture(t, "systems_update_add_delete_1.8.json"), &updates); err != nil {
		t.Fatal(err)
	}
	if len(updates) != 5 {
		t.Fatalf("fixture has %d updates, want 5", len(updates))
	}

	// 1: a new encoder arrives complete.
	mustApplyUpdate(t, st, updates[0])
	sys := mustSnapshot(t, st)
	e6, ok := sys.Encoder("X8A1001-E6")
	if !ok {
		t.Fatal("added encoder X8A1001-E6 not found")
	}
	if len(sys.Encoders) != 6 || !e6.Enabled || e6.Name == "" || e6.Video2110.Port != 5000 {
		t.Errorf("after add: %d encoders, E6 = %+v", len(sys.Encoders), e6)
	}

	// 2: a new decoder arrives complete.
	mustApplyUpdate(t, st, updates[1])
	sys = mustSnapshot(t, st)
	if d6, ok := sys.Decoder("X8A1001-D6"); !ok || !d6.Enabled {
		t.Fatalf("added decoder X8A1001-D6 missing or disabled: %+v", d6)
	}

	// 3: the decoder is disabled; all other fields are kept.
	mustApplyUpdate(t, st, updates[2])
	sys = mustSnapshot(t, st)
	d6, _ := sys.Decoder("X8A1001-D6")
	if d6.Enabled || d6.Name == "" || d6.SignalGenDelay != 60*time.Second {
		t.Errorf("after disable: D6 = %+v, want disabled with name and delay kept", d6)
	}

	// 4 and 5: both units are deleted.
	mustApplyUpdate(t, st, updates[3])
	mustApplyUpdate(t, st, updates[4])
	sys = mustSnapshot(t, st)
	if _, ok := sys.Decoder("X8A1001-D6"); ok {
		t.Error("deleted decoder X8A1001-D6 still present")
	}
	if _, ok := sys.Encoder("X8A1001-E6"); ok {
		t.Error("deleted encoder X8A1001-E6 still present")
	}
	if len(sys.Encoders) != 5 || len(sys.Decoders) != 5 {
		t.Errorf("after delete: %d encoders, %d decoders, want 5/5", len(sys.Encoders), len(sys.Decoders))
	}

	// The heartbeat continues the sequence and only touches link times.
	before, _ := sys.Interface("eth3")
	mustApplyUpdate(t, st, readFixture(t, "systems_update_heartbeat_1.8.json"))
	sys = mustSnapshot(t, st)
	after, _ := sys.Interface("eth3")
	if want := mustTime(t, "2026-10-05 11:20:54 +0000"); !after.LinkSince.Equal(want) {
		t.Errorf("eth3 LinkSince = %v, want %v", after.LinkSince, want)
	}
	after.LinkSince = before.LinkSince
	if after != before {
		t.Errorf("heartbeat changed more than LinkSince:\nbefore %+v\nafter  %+v", before, after)
	}
}

func TestUpdateSRTAndNDI(t *testing.T) {
	st := loadState(t, "systems_full_1.8.json")
	if sys := mustSnapshot(t, st); len(sys.SRTEncoders)+len(sys.SRTDecoders)+len(sys.NDIEncoders)+len(sys.NDIDecoders) != 0 {
		t.Fatal("fixture unexpectedly contains SRT or NDI units")
	}

	var updates []json.RawMessage
	if err := json.Unmarshal(readFixture(t, "systems_update_srt_ndi_1.8.json"), &updates); err != nil {
		t.Fatal(err)
	}
	if len(updates) != 8 {
		t.Fatalf("fixture has %d updates, want 8", len(updates))
	}

	// The first four updates add an NDI encoder and decoder and an SRT
	// encoder and decoder. srt[] and ndi[] mix both directions.
	for _, u := range updates[:4] {
		mustApplyUpdate(t, st, u)
	}
	sys := mustSnapshot(t, st)

	srtEnc, ok := sys.SRTEncoder("X8A1001-srtE1")
	if !ok {
		t.Fatal("SRT encoder X8A1001-srtE1 not found")
	}
	// srtPort and srtLocalPort are strings on the wire.
	wantSRT := SRTConnection{Mode: SRTListener, Port: 7980, LocalPort: 7980}
	if srtEnc.SRT != wantSRT || srtEnc.Bitrate != 10 || !srtEnc.Enabled || srtEnc.Error != "" {
		t.Errorf("SRT encoder = %+v", srtEnc)
	}

	srtDec, ok := sys.SRTDecoder("X8A1001-srtD1")
	if !ok {
		t.Fatal("SRT decoder X8A1001-srtD1 not found")
	}
	if srtDec.SRT.Mode != SRTCaller || srtDec.SRT.Port != 7980 || srtDec.VideoOut != NoSignal {
		t.Errorf("SRT decoder = %+v", srtDec)
	}

	if _, ok := sys.NDIEncoder("X8A1001-NdiE6"); !ok {
		t.Error("NDI encoder X8A1001-NdiE6 not found")
	}
	ndiDec, ok := sys.NDIDecoder("X8A1001-NdiD6")
	if !ok {
		t.Fatal("NDI decoder X8A1001-NdiD6 not found")
	}
	if ndiDec.Source != "" || ndiDec.SourceConnected || ndiDec.FPSSync {
		t.Errorf("NDI decoder = %+v", ndiDec)
	}
	if len(sys.SRTEncoders) != 1 || len(sys.SRTDecoders) != 1 || len(sys.NDIEncoders) != 1 || len(sys.NDIDecoders) != 1 {
		t.Errorf("counts srtE=%d srtD=%d ndiE=%d ndiD=%d, want 1 each",
			len(sys.SRTEncoders), len(sys.SRTDecoders), len(sys.NDIEncoders), len(sys.NDIDecoders))
	}

	// A running state change only carries the changed field.
	partial := []byte(`{"dataid":774,"data":{"local":{"srt":[{"id":"X8A1001-srtE1","values":{"running":true}}]}}}`)
	mustApplyUpdate(t, st, partial)
	if enc, _ := mustSnapshot(t, st).SRTEncoder("X8A1001-srtE1"); !enc.Running || enc.SRT != wantSRT {
		t.Errorf("after running update: %+v", enc)
	}

	// The remaining updates delete all four units again. Their dataids
	// continue after the partial update above.
	for i, u := range updates[4:] {
		var p map[string]json.RawMessage
		if err := json.Unmarshal(u, &p); err != nil {
			t.Fatal(err)
		}
		p["dataid"] = json.RawMessage(fmt.Sprint(775 + i))
		u, _ = json.Marshal(p)
		mustApplyUpdate(t, st, u)
	}
	sys = mustSnapshot(t, st)
	if n := len(sys.SRTEncoders) + len(sys.SRTDecoders) + len(sys.NDIEncoders) + len(sys.NDIDecoders); n != 0 {
		t.Errorf("%d SRT/NDI units left after delete, want 0", n)
	}
}

func TestUpdateGapAndOrder(t *testing.T) {
	var st state
	if _, err := st.applyUpdate([]byte(`{"dataid":2}`)); !errors.Is(err, errNoState) {
		t.Errorf("update before full: err = %v, want errNoState", err)
	}

	if err := st.applyFull([]byte(`{"sysid":"X8A1001","dataid":1,"data":{"local":{"sysid":"X8A1001"}}}`)); err != nil {
		t.Fatal(err)
	}
	if gap, err := st.applyUpdate([]byte(`{"dataid":2,"data":{}}`)); err != nil || gap {
		t.Errorf("consecutive update: gap=%t err=%v, want no gap", gap, err)
	}
	if gap, err := st.applyUpdate([]byte(`{"dataid":5,"data":{}}`)); err != nil || !gap {
		t.Errorf("skipped update: gap=%t err=%v, want gap", gap, err)
	}
	if _, err := st.applyUpdate([]byte(`{"dataid":`)); err == nil {
		t.Error("malformed update: want error")
	}
}

func TestStaleUpdateIgnored(t *testing.T) {
	// Notifications of different methods may be processed out of order. An
	// update that is not newer than the state must not change it.
	var st state
	if err := st.applyFull([]byte(`{"dataid":5,"data":{"local":{"sysid":"X8A1001","name":"new"}}}`)); err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{
		`{"dataid":5,"data":{"local":{"name":"same id"}}}`,
		`{"dataid":3,"data":{"local":{"name":"older"}}}`,
	} {
		if _, err := st.applyUpdate([]byte(stale)); !errors.Is(err, errStaleUpdate) {
			t.Errorf("applyUpdate(%s) = %v, want errStaleUpdate", stale, err)
		}
	}
	if sys := mustSnapshot(t, &st); sys.Name != "new" {
		t.Errorf("Name = %q, want %q", sys.Name, "new")
	}
	mustApplyUpdate(t, &st, []byte(`{"dataid":6,"data":{"local":{"name":"newer"}}}`))
	if sys := mustSnapshot(t, &st); sys.Name != "newer" {
		t.Errorf("Name = %q, want %q", sys.Name, "newer")
	}
}

func TestUpdateForUnknownElement(t *testing.T) {
	// A delta may reference a unit that is not known, e.g. when it was created
	// while updates were lost. The partial element is added and removed again
	// by a later delete.
	var st state
	if err := st.applyFull([]byte(`{"dataid":1,"data":{"local":{"sysid":"X8A1001","dec":[]}}}`)); err != nil {
		t.Fatal(err)
	}
	mustApplyUpdate(t, &st, []byte(`{"dataid":2,"data":{"local":{"dec":[{"id":"X8A1001-D6","enabled":false}]}}}`))
	if sys := mustSnapshot(t, &st); len(sys.Decoders) != 1 {
		t.Fatalf("got %d decoders, want the partial one", len(sys.Decoders))
	}
	mustApplyUpdate(t, &st, []byte(`{"dataid":3,"data":{"local":{"dec":[{"id":"X8A1001-D6","delete":true}]}}}`))
	if sys := mustSnapshot(t, &st); len(sys.Decoders) != 0 {
		t.Errorf("got %d decoders after delete, want 0", len(sys.Decoders))
	}
	// Deleting an unknown element is a no-op.
	mustApplyUpdate(t, &st, []byte(`{"dataid":4,"data":{"local":{"dec":[{"id":"X8A1001-D9","delete":true}]}}}`))
}

func TestInvalidValuesAreReportedOnce(t *testing.T) {
	var st state
	full := `{"dataid":1,"data":{"local":{"sysid":"X8A1001","enc":[` +
		`{"id":"X8A1001-E1","name":"E1","values":{"v2110NetPriPort":"auto","vTBR":25}},` +
		`{"element":"without key"}]}}}`
	if err := st.applyFull([]byte(full)); err != nil {
		t.Fatal(err)
	}

	sys, issues := st.snapshot()
	if len(issues) != 2 {
		t.Fatalf("got %d issues %v, want 2", len(issues), issues)
	}
	wantKeys := map[string]bool{
		"enc.element without key":               true,
		"enc.X8A1001-E1.values.v2110NetPriPort": true,
	}
	for _, issue := range issues {
		if !wantKeys[issue.Key] {
			t.Errorf("unexpected issue key %q", issue.Key)
		}
	}
	if enc, _ := sys.Encoder("X8A1001-E1"); enc.Bitrate != 25 || enc.Video2110.Port != 0 {
		t.Errorf("encoder = bitrate %d port %d, want 25 and 0", enc.Bitrate, enc.Video2110.Port)
	}

	if _, again := st.snapshot(); len(again) != 0 {
		t.Errorf("issues reported again: %v", again)
	}
}

func TestTypeMismatchDoesNotDropMessage(t *testing.T) {
	// A field with an unexpected JSON type is skipped by encoding/json while
	// the rest of the message is still applied.
	var st state
	full := `{"dataid":1,"data":{"local":{"sysid":"X8A1001","name":42,"sysVer":"1.8.4.6"}}}`
	if err := st.applyFull([]byte(full)); err != nil {
		t.Fatalf("applyFull returned error: %v", err)
	}
	sys, _ := st.snapshot()
	if sys.Version != "1.8.4.6" {
		t.Errorf("Version = %q, want 1.8.4.6", sys.Version)
	}
}

func TestSnapshotIsIndependent(t *testing.T) {
	st := loadState(t, "systems_full_1.8.json")
	first := mustSnapshot(t, st)
	first.Encoders[0].Name = "changed"
	first.Peers[0].Encoders = nil

	second := mustSnapshot(t, st)
	if second.Encoders[0].Name == "changed" {
		t.Error("modifying a snapshot changed the state")
	}
}
