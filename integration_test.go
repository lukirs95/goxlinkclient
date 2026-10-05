//go:build integration

// Integration tests against a real VideoXLink system. TestDeviceChanges
// creates, changes and deletes units on the device and links a local decoder
// to an idle encoder of a peer; everything it creates is deleted again. It
// never touches eth0 and never enables outgoing 2110 streams.
//
// Run with:
//
//	XLINK_ADDR=host XLINK_PEER=X8A... XLINK_USER=... XLINK_PASS=... \
//		go test -tags integration -run Device -v -count=1 .
package xlinkclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingHandler records warnings and errors logged by the client.
type recordingHandler struct {
	mu      sync.Mutex
	records []string
}

func (h *recordingHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= slog.LevelWarn }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Level.String() + " " + r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" " + a.String())
		return true
	})
	h.mu.Lock()
	h.records = append(h.records, b.String())
	h.mu.Unlock()
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) all() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.records)
}

type device struct {
	c       *Client
	stats   chan StatsUpdate
	log     *recordingHandler
	peer    SystemID
	runDone chan error
}

// loadDotEnv sets variables from a .env file in the package directory unless
// they are already set. Lines have the form KEY=VALUE.
func loadDotEnv(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || key == "" || strings.HasPrefix(key, "#") {
			continue
		}
		if _, set := os.LookupEnv(key); !set {
			t.Setenv(key, strings.Trim(value, `"'`))
		}
	}
}

// connect runs a client against XLINK_ADDR until the test ends.
func connect(t *testing.T) *device {
	t.Helper()
	loadDotEnv(t)
	addr, user, pass := os.Getenv("XLINK_ADDR"), os.Getenv("XLINK_USER"), os.Getenv("XLINK_PASS")
	if addr == "" || user == "" {
		t.Skip("XLINK_ADDR and XLINK_USER are not set")
	}
	d := &device{
		stats:   make(chan StatsUpdate),
		log:     &recordingHandler{},
		peer:    SystemID(os.Getenv("XLINK_PEER")),
		runDone: make(chan error, 1),
	}
	d.c = New(addr, WithCredentials(user, pass), WithStats(d.stats), WithLogger(slog.New(d.log)))

	ctx, cancel := context.WithCancel(context.Background())
	go func() { d.runDone <- d.c.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-d.runDone; err != nil {
			t.Errorf("Run returned %v", err)
		}
		for _, r := range d.log.all() {
			t.Errorf("client logged: %s", r)
		}
	})

	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer waitCancel()
	if err := d.c.WaitReady(waitCtx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	d.waitFor(t, "first snapshot", func(System) bool { return true })
	return d
}

// waitFor polls the snapshot until ok returns true.
func (d *device) waitFor(t *testing.T, what string, ok func(System) bool) System {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if sys, loaded := d.c.Snapshot(); loaded && ok(sys) {
			return sys
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// nextStats returns the next statistics, skipping ones without health.
func (d *device) nextStats(t *testing.T) Stats {
	t.Helper()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case u := <-d.stats:
			if u.Stats.Health.Uptime > 0 {
				return u.Stats
			}
		case <-timeout:
			t.Fatal("no statistics with health received")
		}
	}
}

// unitIDs returns the IDs of all local units.
func unitIDs(sys System) []UnitID {
	var ids []UnitID
	for _, e := range sys.Encoders {
		ids = append(ids, e.ID)
	}
	for _, d := range sys.Decoders {
		ids = append(ids, d.ID)
	}
	for _, e := range sys.SRTEncoders {
		ids = append(ids, e.ID)
	}
	for _, d := range sys.SRTDecoders {
		ids = append(ids, d.ID)
	}
	for _, e := range sys.NDIEncoders {
		ids = append(ids, e.ID)
	}
	for _, d := range sys.NDIDecoders {
		ids = append(ids, d.ID)
	}
	return ids
}

// createUnit creates a unit and returns its ID. The unit is deleted when the
// test ends.
func (d *device) createUnit(t *testing.T, ctx context.Context, typ UnitType) UnitID {
	t.Helper()
	before, _ := d.c.Snapshot()
	known := unitIDs(before)
	if err := d.c.CreateUnit(ctx, typ); err != nil {
		t.Fatalf("CreateUnit(%v): %v", typ, err)
	}
	var id UnitID
	d.waitFor(t, "new "+typ.String(), func(sys System) bool {
		for _, u := range unitIDs(sys) {
			if u.Type() == typ && !slices.Contains(known, u) {
				id = u
				return true
			}
		}
		return false
	})
	t.Logf("created %v %s", typ, id)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.c.DeleteUnit(ctx, id); err != nil {
			t.Errorf("DeleteUnit(%s): %v", id, err)
			return
		}
		d.waitFor(t, "deletion of "+string(id), func(sys System) bool {
			return !slices.Contains(unitIDs(sys), id)
		})
		t.Logf("deleted %s", id)
	})
	return id
}

// configureInterface refuses to touch eth0, which carries the management
// connection.
func (d *device) configureInterface(t *testing.T, ctx context.Context, name string, settings ...InterfaceSetting) {
	t.Helper()
	if name == "eth0" {
		t.Fatal("refusing to configure eth0")
	}
	if err := d.c.ConfigureInterface(ctx, name, settings...); err != nil {
		t.Fatalf("ConfigureInterface(%s): %v", name, err)
	}
}

func peerUnit(units []PeerUnit, id UnitID) (PeerUnit, bool) {
	return find(units, func(u PeerUnit) bool { return u.ID == id })
}

// unitValues reads the complete configuration of a unit, which may belong to
// a remote system, via state.subscribe.
func (d *device) unitValues(t *testing.T, ctx context.Context, id UnitID) map[string]any {
	t.Helper()
	params := map[string]any{"sysid": d.c.SystemID(), "id": id}
	raw, err := d.c.jrpc.SendRequest(ctx, "state.subscribe", params)
	if err != nil {
		t.Fatalf("state.subscribe(%s): %v", id, err)
	}
	defer d.c.jrpc.SendRequest(ctx, "state.unsubscribe", params)
	var res struct {
		Data struct {
			Values map[string]any `json:"values"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("decode state of %s: %v", id, err)
	}
	return res.Data.Values
}

// stream reads the primary 2110 stream with prefix "v" or "a" from unit
// values as returned by state.subscribe.
func streamFromValues(t *testing.T, v map[string]any, prefix string) Stream {
	t.Helper()
	var s Stream
	s.Interface, _ = v[prefix+"2110NetPri"].(string)
	if s.Interface == "none" {
		s.Interface = ""
	}
	s.Enabled, _ = v[prefix+"2110NetPriEnabled"].(bool)
	s.Address, _ = v[prefix+"2110NetPriIp"].(string)
	var port flexInt
	raw, _ := json.Marshal(v[prefix+"2110NetPriPort"])
	_ = json.Unmarshal(raw, &port)
	s.Port = port.Value
	return s
}

// licensesExhausted reports whether all decoder licenses are in use.
func (d *device) licensesExhausted(t *testing.T) bool {
	t.Helper()
	h := d.nextStats(t).Health
	return h.DecoderLicenses > 0 && h.DecoderLicensesUsed >= h.DecoderLicenses
}

// waitRunning polls until ok returns true and reports whether it did.
func (d *device) waitRunning(timeout time.Duration, ok func(System) bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if sys, loaded := d.c.Snapshot(); loaded && ok(sys) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func TestDeviceChanges(t *testing.T) {
	d := connect(t)
	if d.peer == "" {
		t.Skip("XLINK_PEER is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	t.Run("encoder settings", func(t *testing.T) {
		id := d.createUnit(t, ctx, UnitXLinkEncoder)
		err := d.c.ConfigureEncoder(ctx, id,
			EncoderName("goxlinkclient test"),
			EncoderCodec(VideoCodecH264),
			EncoderBitrate(25),
			EncoderFEC(2),
			EncoderPacketAck(true),
			EncoderMaxRTT(300*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("ConfigureEncoder: %v", err)
		}
		enc := d.waitFor(t, "encoder settings", func(sys System) bool {
			e, ok := sys.Encoder(id)
			return ok && e.Name == "goxlinkclient test" && e.Bitrate == 25
		})
		e, _ := enc.Encoder(id)
		if e.Codec != VideoCodecH264 || e.FECLevel != 2 {
			t.Errorf("encoder = codec %v fec %d", e.Codec, e.FECLevel)
		}

		// H.265 limits the bitrate to 10 Mbps.
		if err := d.c.ConfigureEncoder(ctx, id, EncoderCodec(VideoCodecH265)); err != nil {
			t.Fatalf("ConfigureEncoder(H.265): %v", err)
		}
		d.waitFor(t, "H.265 bitrate limit", func(sys System) bool {
			e, ok := sys.Encoder(id)
			return ok && e.Codec == VideoCodecH265 && e.Bitrate == 10
		})

		// Resetting the buffer of a stopped encoder is rejected.
		if err := d.c.ResetVideoBuffer(ctx, id); err == nil {
			t.Error("ResetVideoBuffer on a stopped encoder succeeded")
		} else {
			t.Logf("ResetVideoBuffer on stopped encoder: %v", err)
		}
	})

	t.Run("decoder linked to peer", func(t *testing.T) {
		id := d.createUnit(t, ctx, UnitXLinkDecoder)
		sys, _ := d.c.Snapshot()
		dec, _ := sys.Decoder(id)
		// Never start a decoder that would send a 2110 stream.
		if dec.Video2110.Enabled || dec.Audio2110.Enabled || dec.Card == VideoCardST2110 {
			t.Fatalf("new decoder has a 2110 output: %+v", dec)
		}

		err := d.c.ConfigureDecoder(ctx, id,
			DecoderName("goxlinkclient test"),
			DecoderSignalGen(true),
			DecoderSignalGenDelay(30*time.Second),
			DecoderFPSSync(false),
		)
		if err != nil {
			t.Fatalf("ConfigureDecoder: %v", err)
		}
		d.waitFor(t, "decoder settings", func(sys System) bool {
			dec, ok := sys.Decoder(id)
			return ok && dec.Name == "goxlinkclient test" && dec.SignalGenOn &&
				dec.SignalGenDelay == 30*time.Second && !dec.FPSSync
		})

		// Pick an idle, unlinked encoder of the peer.
		peer, ok := sys.Peer(d.peer)
		if !ok || !peer.Connected {
			t.Fatalf("peer %s not connected", d.peer)
		}
		var sender PeerUnit
		for _, u := range peer.Encoders {
			if u.Enabled && !u.Running && u.Linked == "" {
				sender = u
				break
			}
		}
		if sender.ID == "" {
			t.Skipf("no idle encoder on %s", d.peer)
		}
		t.Logf("using sender %s %q", sender.ID, sender.Name)

		if err := d.c.ConfigureDecoder(ctx, id, DecoderSender(sender.ID)); err != nil {
			t.Fatalf("ConfigureDecoder(sender): %v", err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = d.c.Stop(ctx, id)
			_ = d.c.Stop(ctx, sender.ID)
			// Unlinking the decoder also clears the sender's receiver.
			if err := d.c.ConfigureDecoder(ctx, id, DecoderSender("")); err != nil {
				t.Errorf("unlink decoder: %v", err)
			}
			d.waitFor(t, "sender restored", func(sys System) bool {
				p, _ := sys.Peer(d.peer)
				u, ok := peerUnit(p.Encoders, sender.ID)
				return ok && u.Linked == "" && !u.Running
			})
			t.Logf("restored %s", sender.ID)
		})
		d.waitFor(t, "link", func(sys System) bool {
			dec, _ := sys.Decoder(id)
			p, _ := sys.Peer(d.peer)
			u, _ := peerUnit(p.Encoders, sender.ID)
			return dec.Sender.ID == sender.ID && u.Linked == id
		})

		if err := d.c.Start(ctx, sender.ID); err != nil {
			t.Fatalf("Start(%s): %v", sender.ID, err)
		}
		if err := d.c.Start(ctx, id); err != nil {
			t.Fatalf("Start(%s): %v", id, err)
		}
		running := d.waitRunning(30*time.Second, func(sys System) bool {
			dec, _ := sys.Decoder(id)
			return dec.Running && dec.Sender.Running
		})
		if !running {
			// The device accepts start but does not run the decoder without a
			// free license.
			if d.licensesExhausted(t) {
				t.Skip("all decoder licenses are in use; the decoder cannot run")
			}
			t.Fatal("linked decoder did not start")
		}

		// The running decoder shows up in the statistics.
		timeout := time.After(20 * time.Second)
		for running := false; !running; {
			select {
			case u := <-d.stats:
				for _, ds := range u.Stats.Decoders {
					if ds.ID == id && ds.Running && ds.Sender.ID == sender.ID {
						t.Logf("stats: %d fps, %.2f Mbps, RTT %v, health %.0f%%, sender running %t",
							ds.OutputFPS, ds.RX, ds.XLink.RTT, ds.Receive.Health, ds.Sender.Running)
						running = true
					}
				}
			case <-timeout:
				t.Fatal("running decoder not in statistics")
			}
		}

		if err := d.c.Stop(ctx, id); err != nil {
			t.Errorf("Stop(%s): %v", id, err)
		}
		if err := d.c.Stop(ctx, sender.ID); err != nil {
			t.Errorf("Stop(%s): %v", sender.ID, err)
		}
		d.waitFor(t, "stopped link", func(sys System) bool {
			dec, _ := sys.Decoder(id)
			return !dec.Running
		})
	})

	t.Run("encoder linked to peer decoder", func(t *testing.T) {
		id := d.createUnit(t, ctx, UnitXLinkEncoder)
		// A new encoder has no input selected and does not run without one.
		// An SDI input without signal is enough if bars are sent instead.
		err := d.c.ConfigureEncoder(ctx, id,
			EncoderName("goxlinkclient test"),
			EncoderBitrate(5),
			EncoderCard(VideoCardSDI1),
			EncoderVideoMode(VideoModeAuto),
			EncoderNoSignal(NoSignalBars),
		)
		if err != nil {
			t.Fatalf("ConfigureEncoder: %v", err)
		}
		d.waitFor(t, "encoder input", func(sys System) bool {
			e, _ := sys.Encoder(id)
			return e.Card == VideoCardSDI1
		})

		// Pick an idle, unlinked decoder of the peer.
		sys, _ := d.c.Snapshot()
		peer, _ := sys.Peer(d.peer)
		var receiver PeerUnit
		for _, u := range peer.Decoders {
			if u.Enabled && !u.Running && u.Linked == "" {
				receiver = u
				break
			}
		}
		if receiver.ID == "" {
			t.Skipf("no idle decoder on %s", d.peer)
		}
		t.Logf("using receiver %s %q", receiver.ID, receiver.Name)

		// Save the receiver's outgoing 2110 streams and switch them off for
		// the test; they are restored exactly afterwards.
		orig := d.unitValues(t, ctx, receiver.ID)
		origVideo, origAudio := streamFromValues(t, orig, "v"), streamFromValues(t, orig, "a")
		origSender, _ := orig["sender"].(string)
		t.Logf("receiver config: card=%v sender=%q video=%+v audio=%+v", orig["vCard"], origSender, origVideo, origAudio)
		if origVideo.Enabled || origAudio.Enabled {
			offVideo, offAudio := origVideo, origAudio
			offVideo.Enabled, offAudio.Enabled = false, false
			if err := d.c.ConfigureDecoder(ctx, receiver.ID, DecoderVideo2110(offVideo), DecoderAudio2110(offAudio)); err != nil {
				t.Fatalf("disable 2110 output of %s: %v", receiver.ID, err)
			}
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = d.c.Stop(ctx, id)
			_ = d.c.Stop(ctx, receiver.ID)
			// Restoring the receiver's sender also unlinks the encoder.
			err := d.c.ConfigureDecoder(ctx, receiver.ID,
				DecoderSender(UnitID(origSender)),
				DecoderVideo2110(origVideo),
				DecoderAudio2110(origAudio),
			)
			if err != nil {
				t.Errorf("restore %s: %v", receiver.ID, err)
				return
			}
			got := d.unitValues(t, ctx, receiver.ID)
			if v, a := streamFromValues(t, got, "v"), streamFromValues(t, got, "a"); v != origVideo || a != origAudio {
				t.Errorf("2110 output of %s not restored: video %+v audio %+v", receiver.ID, v, a)
			} else {
				t.Logf("restored %s: video %+v audio %+v", receiver.ID, v, a)
			}
		})

		// Links are made on the decoder, here a decoder of the peer.
		if err := d.c.ConfigureDecoder(ctx, receiver.ID, DecoderSender(id)); err != nil {
			t.Fatalf("ConfigureDecoder(%s, sender): %v", receiver.ID, err)
		}
		d.waitFor(t, "link", func(sys System) bool {
			e, _ := sys.Encoder(id)
			return e.Receiver.ID == receiver.ID
		})

		if err := d.c.Start(ctx, receiver.ID); err != nil {
			t.Fatalf("Start(%s): %v", receiver.ID, err)
		}
		if err := d.c.Start(ctx, id); err != nil {
			t.Fatalf("Start(%s): %v", id, err)
		}
		running := d.waitRunning(30*time.Second, func(sys System) bool {
			e, _ := sys.Encoder(id)
			return e.Running && e.Receiver.Running
		})
		if !running {
			sys, _ := d.c.Snapshot()
			e, _ := sys.Encoder(id)
			params := map[string]any{"sysid": d.c.SystemID(), "id": id}
			raw, _ := d.c.jrpc.SendRequest(ctx, "state.subscribe", params)
			_, _ = d.c.jrpc.SendRequest(ctx, "state.unsubscribe", params)
			var st struct {
				Data struct {
					Running bool `json:"running"`
				} `json:"data"`
			}
			_ = json.Unmarshal(raw, &st)
			statsRunning := "not reported"
			for _, es := range d.nextStats(t).Encoders {
				if es.ID == id {
					statsRunning = fmt.Sprintf("running=%t fps=%d", es.Running, es.InputFPS)
				}
			}
			t.Fatalf("link did not start: snapshot encoder running=%t xlink=%t; state.subscribe running=%t; stats %s; receiver running=%t connected=%t",
				e.Running, e.XLink, st.Data.Running, statsRunning, e.Receiver.Running, e.Receiver.Connected)
		}

		timeout := time.After(20 * time.Second)
		for seen := false; !seen; {
			select {
			case u := <-d.stats:
				for _, es := range u.Stats.Encoders {
					if es.ID == id && es.Running && es.Receiver.ID == receiver.ID {
						t.Logf("stats: encoder %d fps, RTT %v; receiver running %t, %d fps, %.2f Mbps, health %.0f%%",
							es.InputFPS, es.XLink.RTT, es.Receiver.Running, es.Receiver.OutputFPS, es.Receiver.RX, es.Receiver.Receive.Health)
						seen = true
					}
				}
			case <-timeout:
				t.Fatal("running encoder not in statistics")
			}
		}

		if err := d.c.Stop(ctx, id); err != nil {
			t.Errorf("Stop(%s): %v", id, err)
		}
		if err := d.c.Stop(ctx, receiver.ID); err != nil {
			t.Errorf("Stop(%s): %v", receiver.ID, err)
		}
		d.waitFor(t, "stopped link", func(sys System) bool {
			e, _ := sys.Encoder(id)
			return !e.Running
		})
	})

	t.Run("SRT and NDI units", func(t *testing.T) {
		srtE := d.createUnit(t, ctx, UnitSRTEncoder)
		srtD := d.createUnit(t, ctx, UnitSRTDecoder)
		ndiE := d.createUnit(t, ctx, UnitNDIEncoder)
		ndiD := d.createUnit(t, ctx, UnitNDIDecoder)

		err := d.c.ConfigureSRTEncoder(ctx, srtE,
			SRTEncoderName("goxlinkclient srt"),
			SRTEncoderMode(SRTCaller),
			SRTEncoderAddress("192.0.2.1"),
			SRTEncoderPort(7981),
			SRTEncoderLocalPort(7982),
			SRTEncoderBitrate(8),
		)
		if err != nil {
			t.Fatalf("ConfigureSRTEncoder: %v", err)
		}
		d.waitFor(t, "SRT encoder settings", func(sys System) bool {
			e, ok := sys.SRTEncoder(srtE)
			return ok && e.Name == "goxlinkclient srt" && e.Bitrate == 8 &&
				e.SRT == SRTConnection{Mode: SRTCaller, Address: "192.0.2.1", Port: 7981, LocalPort: 7982}
		})

		if err := d.c.ConfigureSRTDecoder(ctx, srtD, SRTDecoderName("goxlinkclient srt rx"), SRTDecoderMode(SRTListener)); err != nil {
			t.Fatalf("ConfigureSRTDecoder: %v", err)
		}
		d.waitFor(t, "SRT decoder settings", func(sys System) bool {
			dec, ok := sys.SRTDecoder(srtD)
			return ok && dec.Name == "goxlinkclient srt rx" && dec.SRT.Mode == SRTListener
		})

		if err := d.c.ConfigureNDIEncoder(ctx, ndiE, NDIEncoderName("goxlinkclient ndi")); err != nil {
			t.Fatalf("ConfigureNDIEncoder: %v", err)
		}
		if err := d.c.ConfigureNDIDecoder(ctx, ndiD, NDIDecoderName("goxlinkclient ndi rx"), NDIDecoderFPSSync(true)); err != nil {
			t.Fatalf("ConfigureNDIDecoder: %v", err)
		}
		d.waitFor(t, "NDI settings", func(sys System) bool {
			e, okE := sys.NDIEncoder(ndiE)
			dec, okD := sys.NDIDecoder(ndiD)
			return okE && okD && e.Name == "goxlinkclient ndi" && dec.Name == "goxlinkclient ndi rx" && dec.FPSSync
		})
	})

	t.Run("trunk", func(t *testing.T) {
		before, _ := d.c.Snapshot()
		if err := d.c.CreateTrunk(ctx, before.ID); err != nil {
			t.Fatalf("CreateTrunk: %v", err)
		}
		var id TrunkID
		d.waitFor(t, "new trunk", func(sys System) bool {
			for _, tr := range sys.Trunks {
				if _, existed := before.Trunk(tr.ID); !existed {
					id = tr.ID
					return true
				}
			}
			return false
		})
		t.Logf("created trunk %s", id)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := d.c.DeleteTrunk(ctx, id); err != nil {
				t.Errorf("DeleteTrunk(%s): %v", id, err)
				return
			}
			d.waitFor(t, "trunk deletion", func(sys System) bool {
				_, ok := sys.Trunk(id)
				return !ok
			})
			t.Logf("deleted trunk %s", id)
		})

		if err := d.c.ConfigureTrunk(ctx, id, TrunkMTU(1400), TrunkAutoStart(false)); err != nil {
			t.Fatalf("ConfigureTrunk: %v", err)
		}
		d.waitFor(t, "trunk MTU", func(sys System) bool {
			tr, ok := sys.Trunk(id)
			return ok && tr.MTU == 1400 && !tr.AutoStart
		})
	})

	t.Run("interface flags", func(t *testing.T) {
		const name = "eth4"
		sys, _ := d.c.Snapshot()
		orig, ok := sys.Interface(name)
		if !ok || orig.Enabled || orig.LinkUp {
			t.Skipf("%s is not a disabled interface without link", name)
		}
		// The device rejects changes to disabled interfaces ("eth error"), so
		// the interface is enabled for the test. It has no link and carries no
		// traffic.
		d.configureInterface(t, ctx, name, InterfaceEnabled(true))
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			d.configureInterface(t, ctx, name, InterfaceIGMP(orig.IGMP), InterfaceNMOS(orig.NMOS))
			d.configureInterface(t, ctx, name, InterfaceEnabled(false))
			d.waitFor(t, name+" restored", func(sys System) bool {
				i, _ := sys.Interface(name)
				return !i.Enabled && i.IGMP == orig.IGMP && i.NMOS == orig.NMOS
			})
			t.Logf("restored %s", name)
		})
		d.waitFor(t, name+" enabled", func(sys System) bool {
			i, _ := sys.Interface(name)
			return i.Enabled
		})
		d.configureInterface(t, ctx, name, InterfaceIGMP(!orig.IGMP), InterfaceNMOS(!orig.NMOS))
		// Both flags were sent in one configEth request.
		d.waitFor(t, name+" flags", func(sys System) bool {
			i, _ := sys.Interface(name)
			return i.IGMP == !orig.IGMP && i.NMOS == !orig.NMOS
		})
	})
}

func TestDeviceRecon(t *testing.T) {
	d := connect(t)
	sys, _ := d.c.Snapshot()

	t.Logf("system %s %q version %s st2110=%t", sys.ID, sys.Name, sys.Version, sys.ST2110)
	t.Logf("units: %d enc, %d dec, %d srtE, %d srtD, %d ndiE, %d ndiD, %d trunks",
		len(sys.Encoders), len(sys.Decoders), len(sys.SRTEncoders), len(sys.SRTDecoders),
		len(sys.NDIEncoders), len(sys.NDIDecoders), len(sys.Trunks))
	for _, i := range sys.Interfaces {
		t.Logf("  %s enabled=%t link=%t active=%t default=%t backup=%t nmos=%t trunk=%q",
			i.ID, i.Enabled, i.LinkUp, i.Active, i.DefaultExternal, i.Backup, i.NMOS, i.Trunk)
	}
	for _, e := range sys.Encoders {
		t.Logf("  enc %s %q running=%t card=%v receiver=%q", e.ID, e.Name, e.Running, e.Card, e.Receiver.ID)
	}
	for _, dec := range sys.Decoders {
		t.Logf("  dec %s %q running=%t card=%v sender=%q 2110v=%+v", dec.ID, dec.Name, dec.Running, dec.Card, dec.Sender.ID, dec.Video2110)
	}
	for _, p := range sys.Peers {
		t.Logf("  peer %s %q version %s connected=%t p2p=%t", p.ID, p.Name, p.Version, p.Connected, p.P2P)
		if p.ID == d.peer {
			for _, u := range p.Encoders {
				t.Logf("    enc %s %q enabled=%t running=%t linked=%q", u.ID, u.Name, u.Enabled, u.Running, u.Linked)
			}
			for _, u := range p.Decoders {
				t.Logf("    dec %s %q enabled=%t running=%t linked=%q", u.ID, u.Name, u.Enabled, u.Running, u.Linked)
			}
		}
	}

	s := d.nextStats(t)
	t.Logf("stats: health %+v", s.Health)
	t.Logf("stats: %d interfaces, %d enc, %d dec, %d peers", len(s.Interfaces), len(s.Encoders), len(s.Decoders), len(s.Peers))
}
