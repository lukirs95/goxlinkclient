package xlinkclient

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestDecodeLocalStats(t *testing.T) {
	ls, issues, err := decodeLocalStats(readFixture(t, "stats_local_1.8.json"))
	if err != nil {
		t.Fatalf("decodeLocalStats: %v", err)
	}
	for _, issue := range issues {
		t.Errorf("unexpected decode issue: %v", issue)
	}

	if ls.System != "X8A1001" || !ls.Time.Equal(mustTime(t, "2026-10-05 13:12:01 +0000")) {
		t.Errorf("System %s Time %v", ls.System, ls.Time)
	}
	// The system item carries no data in systems.localStats; only active
	// interfaces are reported.
	if len(ls.Interfaces) != 4 || len(ls.Encoders) != 5 || len(ls.Decoders) != 5 {
		t.Fatalf("counts nets=%d enc=%d dec=%d, want 4/5/5", len(ls.Interfaces), len(ls.Encoders), len(ls.Decoders))
	}
	if eth2 := ls.Interfaces[2]; eth2.ID != "eth2" || eth2.RX != 11.64 || eth2.TX != 0.5 {
		t.Errorf("eth2 = %+v", eth2)
	}

	// D1 is running: its buffer statistics are strings on the wire.
	d1 := findStats(t, ls.Decoders, "X8A1001-D1")
	if !d1.Running || d1.Uptime != 87*time.Second || d1.OutputFPS != 25 || d1.RX != 10.13 {
		t.Errorf("D1 = running %t uptime %v fps %v rx %v", d1.Running, d1.Uptime, d1.OutputFPS, d1.RX)
	}
	if d1.Buffer != (BufferStats{Delay: 100 * time.Millisecond, HeadMin: 100 * time.Millisecond}) {
		t.Errorf("D1 Buffer = %+v", d1.Buffer)
	}
	if d1.Video != (VideoStats{FECFailed: 11}) || d1.Audio != (AudioStats{Dropped: 2, Unsynced: 1}) {
		t.Errorf("D1 Video %+v Audio %+v", d1.Video, d1.Audio)
	}
	// "late" in systems.localStats is not the packet count of systems.stats.
	if d1.Packets != (PacketStats{}) || d1.StatsTime != 0 || d1.DecodeFPS != 0 {
		t.Errorf("D1 has values of systems.stats: packets %+v stats time %v decode fps %v", d1.Packets, d1.StatsTime, d1.DecodeFPS)
	}
	if s := d1.Sender; s.ID != "X8A1008-E5" || !s.Running || s.InputFPS != 25 || s.XLink.RTT != 140*time.Microsecond || s.Uptime != 0 {
		t.Errorf("D1 Sender = %+v", s)
	}

	// Idle units report numbers; an encoder carries its receiver.
	e1 := ls.Encoders[0]
	if e1.ID != "X8A1001-E1" || e1.Running || e1.Receiver.ID != "X4A1004-D1" || e1.Receiver.Running {
		t.Errorf("E1 = %+v", e1)
	}
}

func TestDecodeSystemStats(t *testing.T) {
	ss, issues, err := decodeSystemStats(readFixture(t, "stats_system_1.8.json"))
	if err != nil {
		t.Fatalf("decodeSystemStats: %v", err)
	}
	for _, issue := range issues {
		t.Errorf("unexpected decode issue: %v", issue)
	}

	want := Health{
		PTP: true, PTPSync: true, NMOS: true, Uptime: 15120 * time.Second,
		CPU: 2, CPUTemp: 63, Licenses: 8, DecoderLicenses: 4, DecoderLicensesUsed: 1,
	}
	if ss.health != want {
		t.Errorf("Health = %+v, want %+v", ss.health, want)
	}

	if len(ss.peers) != 1 {
		t.Fatalf("got %d peers, want 1", len(ss.peers))
	}
	// The remote RTT is a string on the wire.
	if p := ss.peers[0]; p.ID != "X8A1008" || p.Health.CPUTemp != 52 || p.RTT != 201*time.Microsecond {
		t.Errorf("peer = %+v", p)
	}
	// XLink units of the local and the remote system are kept.
	if ss.decoders["X8A1001-D1"] == nil || ss.encoders["X8A1008-E5"] == nil {
		t.Errorf("units missing: decoders %d encoders %d", len(ss.decoders), len(ss.encoders))
	}
}

func TestMergeStats(t *testing.T) {
	ls, _, err := decodeLocalStats(readFixture(t, "stats_local_1.8.json"))
	if err != nil {
		t.Fatalf("decodeLocalStats: %v", err)
	}
	ss, _, err := decodeSystemStats(readFixture(t, "stats_system_1.8.json"))
	if err != nil {
		t.Fatalf("decodeSystemStats: %v", err)
	}
	s, issues := mergeStats(ls, ss)
	for _, issue := range issues {
		t.Errorf("unexpected decode issue: %v", issue)
	}

	if s.Health.CPUTemp != 63 || len(s.Peers) != 1 || len(s.SRTEncoders) != 0 || len(s.SRTDecoders) != 0 {
		t.Errorf("Health %+v peers %d srt %d/%d", s.Health, len(s.Peers), len(s.SRTEncoders), len(s.SRTDecoders))
	}

	// The local decoder keeps its values and gains those of systems.stats.
	d1 := findStats(t, s.Decoders, "X8A1001-D1")
	if d1.Uptime != 87*time.Second || d1.StatsTime != 83*time.Second || d1.DecodeFPS != 25 || d1.OutputFPS != 25 {
		t.Errorf("D1 uptime %v stats time %v fps %v/%v", d1.Uptime, d1.StatsTime, d1.DecodeFPS, d1.OutputFPS)
	}
	if want := (VideoStats{Total: 2000, FECOK: 1969, FECFailed: 11}); d1.Video != want {
		t.Errorf("D1 Video = %+v, want %+v", d1.Video, want)
	}
	if d1.Buffer.Delay != 100*time.Millisecond {
		t.Errorf("D1 Buffer = %+v", d1.Buffer)
	}

	// Its sender on the remote system gains uptime, ARQ details and audio.
	sender := d1.Sender
	if sender.Uptime != 85*time.Second || sender.StatsTime != 85*time.Second || sender.InputFPS != 25 {
		t.Errorf("Sender uptime %v stats time %v fps %v", sender.Uptime, sender.StatsTime, sender.InputFPS)
	}
	wantXLink := EncoderXLinkStats{RTT: 140 * time.Microsecond, ACK: true, Cloud: 25}
	if sender.XLink != wantXLink {
		t.Errorf("Sender XLink = %+v, want %+v", sender.XLink, wantXLink)
	}
	if len(sender.Audio) != 16 || sender.Audio[0] != (AudioLevel{Channel: 1, Level: -22.01, Peak: -18}) ||
		!slices.IsSortedFunc(sender.Audio, func(a, b AudioLevel) int { return a.Channel - b.Channel }) {
		t.Errorf("Sender Audio = %+v", sender.Audio)
	}

	// The receiver of E1 is on a system missing from systems.stats.
	e1 := findStats(t, s.Encoders, "X8A1001-E1")
	if !e1.XLink.ACK || len(e1.Audio) != 0 || e1.Receiver.StatsTime != 0 {
		t.Errorf("E1 = %+v", e1)
	}
}

func TestMergeStatsSRT(t *testing.T) {
	ss, issues, err := decodeSystemStats(readFixture(t, "stats_system_srt_1.8.json"))
	if err != nil {
		t.Fatalf("decodeSystemStats: %v", err)
	}
	s, mergeIssues := mergeStats(localStats{}, ss)
	for _, issue := range append(issues, mergeIssues...) {
		t.Errorf("unexpected decode issue: %v", issue)
	}
	if len(s.SRTEncoders) != 1 || len(s.SRTDecoders) != 1 {
		t.Fatalf("SRT units %d/%d, want 1/1", len(s.SRTEncoders), len(s.SRTDecoders))
	}

	// A listener reports one connection per caller.
	enc := s.SRTEncoders[0]
	wantEnc := SRTEncoderStats{
		ID: "X8A1001-srtE1", Uptime: 204 * time.Second, StatsTime: 193 * time.Second, Clients: 1,
		Connections: []SRTSendStats{{
			Peer: "10.0.0.2", Bitrate: 7.92725, RTT: 22 * time.Microsecond, Latency: 250 * time.Millisecond,
			Sent: 107222, ACK: 8383,
		}},
	}
	if enc.ID != wantEnc.ID || enc.Uptime != wantEnc.Uptime || enc.StatsTime != wantEnc.StatsTime ||
		enc.Clients != wantEnc.Clients || !slices.Equal(enc.Connections, wantEnc.Connections) {
		t.Errorf("SRT encoder = %+v, want %+v", enc, wantEnc)
	}

	// A caller reports its connection directly.
	dec := s.SRTDecoders[0]
	wantConn := SRTReceiveStats{
		Bitrate: 7.92164, RTT: 17 * time.Microsecond, Latency: 250 * time.Millisecond,
		Received: 107406, ACK: 8404,
	}
	if dec.ID != "X8A1001-srtD1" || dec.Clients != 0 || len(dec.Connections) != 1 || dec.Connections[0] != wantConn {
		t.Errorf("SRT decoder = %+v, want connection %+v", dec, wantConn)
	}

	// Every Stats value has its own slices.
	again, _ := mergeStats(localStats{}, ss)
	again.SRTEncoders[0].Connections[0].Sent = 0
	if s.SRTEncoders[0].Connections[0].Sent != 107222 {
		t.Error("Stats values share SRT connections")
	}
}

func TestDecodeStatsInvalidValues(t *testing.T) {
	params := []byte(`{"sysid":"X8A1001","time":"2026-10-05 13:12:01 +0000","data":[` +
		`{"id":"eth0","type":0,"data":{"rx":"fast","tx":1}},` +
		`{"id":"X8A1001-D1","type":2,"data":{"running":true,"vOutFps":29.97,"vRstats":{"head_min_ms":"many","delay":"100"}}},` +
		`{"id":"X8A1001-srtE1","type":8,"data":{"running":true}},` +
		`{"id":"X8A1001-E1","type":1,"data":"broken"}]}`)
	ls, issues, err := decodeLocalStats(params)
	if err != nil {
		t.Fatalf("decodeLocalStats: %v", err)
	}
	// Invalid values are reported, the rest is decoded, unknown unit types
	// are skipped.
	wantKeys := map[string]bool{
		"eth0.rx":                        true,
		"X8A1001-D1.vRstats.head_min_ms": true,
		"X8A1001-E1.data":                true,
	}
	if len(issues) != len(wantKeys) {
		t.Fatalf("issues = %v, want keys %v", issues, wantKeys)
	}
	for _, issue := range issues {
		if !wantKeys[issue.Key] {
			t.Errorf("unexpected issue key %q", issue.Key)
		}
	}
	if len(ls.Interfaces) != 1 || ls.Interfaces[0].TX != 1 {
		t.Errorf("Interfaces = %+v", ls.Interfaces)
	}
	// Frame rates may be fractional.
	if len(ls.Decoders) != 1 || ls.Decoders[0].Buffer.Delay != 100*time.Millisecond || !ls.Decoders[0].Running ||
		ls.Decoders[0].OutputFPS != 29.97 {
		t.Errorf("Decoders = %+v", ls.Decoders)
	}
	if len(ls.Encoders) != 0 {
		t.Errorf("Encoders = %+v, want none", ls.Encoders)
	}

	if _, _, err := decodeLocalStats([]byte(`[`)); !errors.Is(err, errBadStats) {
		t.Errorf("malformed message: err = %v, want errBadStats", err)
	}
}

func TestMergeStatsInvalidValues(t *testing.T) {
	ss, issues, err := decodeSystemStats([]byte(`{"sysid":"X8A1001","data":{"local":[` +
		`{"id":"X8A1001-D1","type":2,"data":{"vDecFps":"slow","vDstats":{"total":5}}},` +
		`{"id":"X8A1001-srtD1","type":9,"data":{"upTime":1,"srtStats":{"mode":"Caller","msRTT":"?"}}}],"remote":[]}}`))
	if err != nil || len(issues) != 0 {
		t.Fatalf("decodeSystemStats: %v, issues %v", err, issues)
	}
	ls := localStats{Decoders: []DecoderStats{{DecoderMetrics: DecoderMetrics{ID: "X8A1001-D1"}}}}

	// Invalid values are reported when they are merged, once.
	s, issues := mergeStats(ls, ss)
	wantKeys := []string{"X8A1001-D1.vDecFps", "X8A1001-srtD1.srtStats.msRTT"}
	if len(issues) != len(wantKeys) || issues[0].Key != wantKeys[0] || issues[1].Key != wantKeys[1] {
		t.Errorf("issues = %v, want keys %v", issues, wantKeys)
	}
	if s.Decoders[0].Video.Total != 5 || len(s.SRTDecoders) != 1 || s.SRTDecoders[0].Uptime != time.Second {
		t.Errorf("Stats = %+v", s)
	}
	if _, issues := mergeStats(ls, ss); len(issues) != 0 {
		t.Errorf("issues reported again: %v", issues)
	}
}

// findStats returns the encoder or decoder statistics with the given ID.
func findStats[T EncoderStats | DecoderStats](t *testing.T, units []T, id UnitID) T {
	t.Helper()
	for _, u := range units {
		switch v := any(u).(type) {
		case EncoderStats:
			if v.ID == id {
				return u
			}
		case DecoderStats:
			if v.ID == id {
				return u
			}
		}
	}
	t.Fatalf("unit %s not found", id)
	var zero T
	return zero
}
