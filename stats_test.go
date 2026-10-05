package xlinkclient

import (
	"errors"
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

	// D1 is running: its receive statistics are strings on the wire.
	var d1 DecoderStats
	for _, d := range ls.Decoders {
		if d.ID == "X8A1001-D1" {
			d1 = d
		}
	}
	if !d1.Running || d1.Uptime != 87*time.Second || d1.OutputFPS != 25 || d1.RX != 10.13 {
		t.Errorf("D1 = running %t uptime %v fps %d rx %v", d1.Running, d1.Uptime, d1.OutputFPS, d1.RX)
	}
	if d1.XLink.RTT != 180*time.Microsecond {
		t.Errorf("D1 RTT = %v, want 180µs (0.18 ms)", d1.XLink.RTT)
	}
	wantReceive := ReceiveStats{
		Delay: 100 * time.Millisecond, Expected: 1300, Received: 1300,
		JitterAvg: 402 * time.Microsecond, Health: 100, TotalHealth: 100,
	}
	got := d1.Receive
	got.JitterMin, got.JitterMax = 0, 0
	if got != wantReceive {
		t.Errorf("D1 Receive = %+v, want %+v", got, wantReceive)
	}
	if d1.Video != (VideoStats{Corrected: 11, FECFailed: 11}) || d1.Audio != (AudioStats{Dropped: 2, Unsynced: 1}) {
		t.Errorf("D1 Video %+v Audio %+v", d1.Video, d1.Audio)
	}
	if d1.Buffer != (BufferStats{Size: 100 * time.Millisecond}) {
		t.Errorf("D1 Buffer = %+v", d1.Buffer)
	}
	if d1.Sender.ID != "X8A1008-E5" || !d1.Sender.Running || d1.Sender.InputFPS != 25 || d1.Sender.XLink.P2P != 369761 {
		t.Errorf("D1 Sender = %+v", d1.Sender)
	}

	// Idle units report numbers; an encoder carries its receiver.
	e1 := ls.Encoders[0]
	if e1.ID != "X8A1001-E1" || e1.Running || e1.Receiver.ID != "X4A1004-D1" || e1.Receiver.Receive.Health != 100 {
		t.Errorf("E1 = %+v", e1)
	}
}

func TestDecodeSystemStats(t *testing.T) {
	health, peers, issues, err := decodeSystemStats(readFixture(t, "stats_system_1.8.json"))
	if err != nil {
		t.Fatalf("decodeSystemStats: %v", err)
	}
	for _, issue := range issues {
		t.Errorf("unexpected decode issue: %v", issue)
	}

	want := Health{
		PTP: true, PTPSync: true, NMOS: true, Uptime: 15120 * time.Second,
		CPU: 2, CPUTemp: 63, RunningVideos: 1, Licenses: 8, DecoderLicenses: 4, DecoderLicensesUsed: 1,
	}
	if health != want {
		t.Errorf("Health = %+v, want %+v", health, want)
	}

	if len(peers) != 1 {
		t.Fatalf("got %d peers, want 1", len(peers))
	}
	// The remote RTT is a string on the wire.
	if p := peers[0]; p.ID != "X8A1008" || p.Health.CPUTemp != 52 || p.RTT != 201*time.Microsecond {
		t.Errorf("peer = %+v", p)
	}
}

func TestDecodeStatsInvalidValues(t *testing.T) {
	params := []byte(`{"sysid":"X8A1001","time":"2026-10-05 13:12:01 +0000","data":[` +
		`{"id":"eth0","type":0,"data":{"rx":"fast","tx":1}},` +
		`{"id":"X8A1001-D1","type":2,"data":{"running":true,"vRstats":{"received":"many","delay":"100"}}},` +
		`{"id":"X8A1001-srtE1","type":8,"data":{"running":true}},` +
		`{"id":"X8A1001-E1","type":1,"data":"broken"}]}`)
	ls, issues, err := decodeLocalStats(params)
	if err != nil {
		t.Fatalf("decodeLocalStats: %v", err)
	}
	// Invalid values are reported, the rest is decoded, unknown unit types
	// are skipped.
	wantKeys := map[string]bool{
		"eth0.rx":                     true,
		"X8A1001-D1.vRstats.received": true,
		"X8A1001-E1.data":             true,
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
	if len(ls.Decoders) != 1 || ls.Decoders[0].Receive.Delay != 100*time.Millisecond || !ls.Decoders[0].Running {
		t.Errorf("Decoders = %+v", ls.Decoders)
	}
	if len(ls.Encoders) != 0 {
		t.Errorf("Encoders = %+v, want none", ls.Encoders)
	}

	if _, _, err := decodeLocalStats([]byte(`[`)); !errors.Is(err, errBadStats) {
		t.Errorf("malformed message: err = %v, want errBadStats", err)
	}
}
