package xlinkclient

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"
)

// loadConfigs applies the state.subscribe responses of the fixture.
func loadConfigs(t *testing.T) configs {
	t.Helper()
	var responses []json.RawMessage
	if err := json.Unmarshal(readFixture(t, "state_subscribe_1.8.json"), &responses); err != nil {
		t.Fatal(err)
	}
	cs := configs{}
	for _, r := range responses {
		if _, err := subscribeRaw(cs, r); err != nil {
			t.Fatalf("applySubscribe: %v", err)
		}
	}
	return cs
}

// config converts the configuration of unit id with conv.
func config[T any](t *testing.T, cs configs, id UnitID, conv func(*wireConfigValues, *collector) T) T {
	t.Helper()
	v, ok := cs.get(id)
	if !ok {
		t.Fatalf("configuration of %s not loaded", id)
	}
	var c collector
	cfg := conv(v, &c)
	for _, issue := range c.issues {
		t.Errorf("unexpected decode issue: %v", issue)
	}
	return cfg
}

func TestUnitConfigs(t *testing.T) {
	cs := loadConfigs(t)

	// vCard is a string, vTBR a float, ports are numbers.
	enc := config(t, cs, "X8A1001-E1", (*wireConfigValues).encoder)
	if enc.Name != "Encoder 106" || !enc.AutoStart || enc.Codec != VideoCodecH264 || enc.BitDepth != 10 ||
		enc.Chroma != 422 || !enc.IntraRefresh || !enc.Interlaced || enc.CRFOn || enc.CRF != 10 ||
		!enc.GOPOn || enc.GOP != 25 || enc.FECLevel != 1 || enc.Bitrate != 18 {
		t.Errorf("encoder video = %+v", enc)
	}
	if enc.AudioMode != AudioCompressed || enc.AudioBitDepth != 32 || enc.AudioChannels != 16 || !enc.AudioFEC ||
		enc.AudioBitrate != 128 || enc.Card != VideoCardST2110 || enc.VideoMode != "Hi50" || enc.VideoLock != VideoModeAuto {
		t.Errorf("encoder audio/source = %+v", enc)
	}
	if enc.Video2110 != (Stream{Interface: "eth6", Enabled: true, Address: "239.162.105.5", Port: 30000}) ||
		enc.Video2110Payload != 97 || enc.Audio2110Channels != 8 || enc.Audio2110PacketTime != 1 {
		t.Errorf("encoder 2110 = %+v", enc)
	}
	if !enc.PacketAck || !enc.Resend || enc.MaxResends != 3 || enc.ResendDelay != 80*time.Millisecond ||
		enc.MaxRTT != 160*time.Millisecond || enc.Uplink != "" {
		t.Errorf("encoder XLink = %+v", enc)
	}

	dec := config(t, cs, "X8A1001-D1", (*wireConfigValues).decoder)
	if dec.Name != "Decoder 106" || dec.Sender != "X8A1008-E1" || !dec.AVSync || !dec.FPSSync ||
		dec.PacketBuffer != 700*time.Millisecond || dec.AllowLateFrames || dec.SignalGenDelay != 60*time.Second ||
		dec.SignalGenType != SignalGeneratorBlack || !dec.OverlayTextOn || dec.OverlayText != "%sysname% | Receiver %name% | %format%" ||
		dec.Card != VideoCardST2110 || dec.Video2110Payload != 96 || dec.Audio2110PacketTime != 2 {
		t.Errorf("decoder = %+v", dec)
	}

	srtEnc := config(t, cs, "X8A1001-srtE1", (*wireConfigValues).srtEncoder)
	wantSRTEnc := SRTEncoderConfig{
		Name: "SRT S1", AutoStart: true, Mode: SRTListener, Address: "203.0.113.1", Port: 5004, LocalPort: 7980,
		Latency: 250 * time.Millisecond, KeyLength: SRTAES128, BitDepth: 10, GOP: 15, Bitrate: 20, AudioBitrate: 128,
		VideoMode: "Hi50", AudioChannels: 8,
	}
	if srtEnc != wantSRTEnc {
		t.Errorf("SRT encoder = %+v\nwant          %+v", srtEnc, wantSRTEnc)
	}

	srtDec := config(t, cs, "X8A1001-srtD1", (*wireConfigValues).srtDecoder)
	wantSRTDec := SRTDecoderConfig{
		Name: "SRT R1", Mode: SRTCaller, Address: "10.0.0.2", Port: 7980, LocalPort: 7990,
		Latency: 250 * time.Millisecond, KeyLength: SRTAES128, Audio: true, AudioCodec: SRTAudioAAC,
	}
	if srtDec != wantSRTDec {
		t.Errorf("SRT decoder = %+v\nwant          %+v", srtDec, wantSRTDec)
	}

	ndiEnc := config(t, cs, "X8A1001-NdiE7", (*wireConfigValues).ndiEncoder)
	if want := (NDIEncoderConfig{Name: "NDI S7", Card: 2, VideoMode: VideoModeAuto, BarsStandard: VideoModeAuto, AudioChannels: 2}); ndiEnc != want {
		t.Errorf("NDI encoder = %+v, want %+v", ndiEnc, want)
	}

	ndiDec := config(t, cs, "X8A1001-NdiD6", (*wireConfigValues).ndiDecoder)
	if want := (NDIDecoderConfig{Name: "NDI R6", AVSync: true, AudioChannels: 2, Card: 4, VideoMode: VideoModeAuto}); ndiDec != want {
		t.Errorf("NDI decoder = %+v, want %+v", ndiDec, want)
	}
}

func TestConfigUpdates(t *testing.T) {
	cs := loadConfigs(t)
	update := func(dataid int, values string) json.RawMessage {
		return json.RawMessage(`{"id":"X8A1001-E1","dataid":` + strconv.Itoa(dataid) + `,"data":{"values":` + values + `}}`)
	}
	intraRefresh := func() bool {
		v, _ := cs.get("X8A1001-E1")
		return v.VPIntraOn.Value
	}

	// A delta changes only the keys it carries.
	if gap, err := updateRaw(cs, update(2, `{"vPIntraOn":false}`)); err != nil || gap || intraRefresh() {
		t.Fatalf("update 2: gap %t err %v intra %t", gap, err, intraRefresh())
	}
	if v, _ := cs.get("X8A1001-E1"); v.Name != "Encoder 106" || v.VTBR.Value != 18 {
		t.Errorf("unchanged values lost: %+v", v)
	}
	// Old updates are ignored, lost ones reported.
	if _, err := updateRaw(cs, update(2, `{"vPIntraOn":true}`)); !errors.Is(err, errStaleUpdate) || intraRefresh() {
		t.Errorf("stale update: err %v intra %t", err, intraRefresh())
	}
	if gap, err := updateRaw(cs, update(5, `{"vPIntraOn":true}`)); err != nil || !gap || !intraRefresh() {
		t.Errorf("update 5: gap %t err %v intra %t", gap, err, intraRefresh())
	}
	if _, err := updateRaw(cs, json.RawMessage(`{"id":"X8A1001-E9","dataid":2,"data":{}}`)); !errors.Is(err, errUnknownUnit) {
		t.Errorf("unknown unit: err %v", err)
	}

	// Updates arriving before the response are applied after it, unless the
	// response is newer.
	cs = configs{}
	cs.expect("X8A1001-E1")
	if _, ok := cs.get("X8A1001-E1"); ok {
		t.Fatal("expected unit is loaded")
	}
	for _, u := range []json.RawMessage{update(1, `{"vTBR":5}`), update(2, `{"vPIntraOn":false}`)} {
		if _, err := updateRaw(cs, u); err != nil {
			t.Fatalf("pending update: %v", err)
		}
	}
	var all []json.RawMessage
	if err := json.Unmarshal(readFixture(t, "state_subscribe_1.8.json"), &all); err != nil {
		t.Fatal(err)
	}
	if _, err := subscribeRaw(cs, all[0]); err != nil {
		t.Fatal(err)
	}
	if v, _ := cs.get("X8A1001-E1"); v.VTBR.Value != 18 || v.VPIntraOn.Value || cs["X8A1001-E1"].dataID != 2 {
		t.Errorf("after response: vTBR %d intra %t dataid %d, want 18/false/2", v.VTBR.Value, v.VPIntraOn.Value, cs["X8A1001-E1"].dataID)
	}
}

// subscribeRaw applies an encoded state.subscribe response.
func subscribeRaw(cs configs, raw json.RawMessage) (UnitID, error) {
	m, err := decodeConfigMessage(raw)
	if err != nil {
		return "", err
	}
	return UnitID(m.ID), cs.applySubscribe(&m)
}

// updateRaw applies an encoded state.update.
func updateRaw(cs configs, raw json.RawMessage) (gap bool, err error) {
	m, err := decodeConfigMessage(raw)
	if err != nil {
		return false, err
	}
	_, gap, err = cs.applyUpdate(&m, raw)
	return gap, err
}
