//go:build integration

// Integration tests against a real VideoXLink system. TestDeviceChanges
// creates, changes and deletes units, a trunk and interface flags on the
// device and links units to idle units of a peer; everything it changes is
// restored afterwards. It never touches eth0, eth6 or eth7, never enables
// outgoing 2110 streams and never reboots or shuts down a system.
//
// The test follows the updates the client delivers: every action waits for
// the update of the element it changed, which carries the complete element.
//
// Run with:
//
//	XLINK_ADDR=host XLINK_PEER=X8A... XLINK_USER=... XLINK_PASS=... \
//		go test -tags integration -run Device -v -count=1 .
package xlinkclient

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// protectedInterfaces must never be changed by the tests.
var protectedInterfaces = []string{"eth0", "eth6", "eth7"}

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

// device is a client connected to XLINK_ADDR that records every update.
type device struct {
	c       *Client
	stats   chan StatsUpdate
	log     *recordingHandler
	peer    SystemID
	runDone chan error

	mu      sync.Mutex
	updates []Update
	changed chan struct{} // closed and replaced when an update arrives
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

// connect runs a client against XLINK_ADDR until the test ends and waits for
// the first snapshot.
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
		changed: make(chan struct{}),
	}
	updates := make(chan Update)
	d.c = New(addr, WithCredentials(user, pass), WithUpdates(updates), WithStats(d.stats), WithLogger(slog.New(d.log)))

	ctx, cancel := context.WithCancel(context.Background())
	go d.record(ctx, updates)
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
	snap := d.waitUpdate(t, 0, "the snapshot", func(u Update) bool { return u.Kind == UpdateSnapshot })
	if snap.Snapshot == nil || snap.Snapshot.ID != d.c.SystemID() {
		t.Fatalf("snapshot = %+v", snap)
	}
	return d
}

// record stores the updates of ch until ctx is done.
func (d *device) record(ctx context.Context, ch <-chan Update) {
	for {
		select {
		case <-ctx.Done():
			return
		case u := <-ch:
			d.mu.Lock()
			d.updates = append(d.updates, u)
			close(d.changed)
			d.changed = make(chan struct{})
			d.mu.Unlock()
		}
	}
}

// mark returns the position after the updates received so far.
func (d *device) mark() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.updates)
}

// waitUpdate returns the first update after position from that satisfies ok.
func (d *device) waitUpdate(t *testing.T, from int, what string, ok func(Update) bool) Update {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		d.mu.Lock()
		list, changed := d.updates[from:], d.changed
		d.mu.Unlock()
		for _, u := range list {
			if ok(u) {
				return u
			}
		}
		from += len(list)
		select {
		case <-changed:
		case <-timeout:
			t.Fatalf("no update with %s", what)
			return Update{}
		}
	}
}

// Predicates for waitUpdate.

func encoderWhere(id UnitID, ok func(*Encoder) bool) func(Update) bool {
	return func(u Update) bool {
		return u.Kind == UpdateEncoder && u.ID == string(id) && u.Encoder != nil && ok(u.Encoder)
	}
}

func decoderWhere(id UnitID, ok func(*Decoder) bool) func(Update) bool {
	return func(u Update) bool {
		return u.Kind == UpdateDecoder && u.ID == string(id) && u.Decoder != nil && ok(u.Decoder)
	}
}

func peerWhere(id SystemID, ok func(*Peer) bool) func(Update) bool {
	return func(u Update) bool {
		return u.Kind == UpdatePeer && u.ID == string(id) && u.Peer != nil && ok(u.Peer)
	}
}

func removed(kind UpdateKind, id string) func(Update) bool {
	return func(u Update) bool { return u.Kind == kind && u.ID == id && u.Removed }
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

// snapshot returns the current state.
func (d *device) snapshot(t *testing.T) System {
	t.Helper()
	sys, ok := d.c.Snapshot()
	if !ok {
		t.Fatal("no snapshot")
	}
	return sys
}

// updateKind returns the kind of the updates of a unit type.
func updateKind(typ UnitType) UpdateKind {
	switch typ {
	case UnitXLinkEncoder:
		return UpdateEncoder
	case UnitXLinkDecoder:
		return UpdateDecoder
	case UnitSRTEncoder:
		return UpdateSRTEncoder
	case UnitSRTDecoder:
		return UpdateSRTDecoder
	case UnitNDIEncoder:
		return UpdateNDIEncoder
	default:
		return UpdateNDIDecoder
	}
}

// createUnit creates a unit and returns its ID. The unit is deleted when the
// test ends.
func (d *device) createUnit(t *testing.T, ctx context.Context, typ UnitType) UnitID {
	t.Helper()
	from := d.mark()
	if err := d.c.CreateUnit(ctx, typ); err != nil {
		t.Fatalf("CreateUnit(%v): %v", typ, err)
	}
	// A new unit arrives as an update of a unit unknown before.
	before := d.snapshotBefore(from)
	u := d.waitUpdate(t, from, "new "+typ.String(), func(u Update) bool {
		return u.Kind == updateKind(typ) && !u.Removed && !before[u.ID]
	})
	id := UnitID(u.ID)
	t.Logf("created %v %s", typ, id)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		from := d.mark()
		if err := d.c.DeleteUnit(ctx, id); err != nil {
			t.Errorf("DeleteUnit(%s): %v", id, err)
			return
		}
		d.waitUpdate(t, from, "deletion of "+string(id), removed(updateKind(typ), string(id)))
		t.Logf("deleted %s", id)
	})
	return id
}

// snapshotBefore returns the IDs of the elements reported before position
// from.
func (d *device) snapshotBefore(from int) map[string]bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	known := map[string]bool{}
	for _, u := range d.updates[:from] {
		if u.Kind == UpdateSnapshot {
			for _, id := range unitIDs(*u.Snapshot) {
				known[string(id)] = true
			}
			continue
		}
		known[u.ID] = !u.Removed
	}
	return known
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

// configureInterface refuses to touch the protected interfaces.
func (d *device) configureInterface(t *testing.T, ctx context.Context, name string, settings ...InterfaceSetting) {
	t.Helper()
	if slices.Contains(protectedInterfaces, name) {
		t.Fatalf("refusing to configure %s", name)
	}
	if err := d.c.ConfigureInterface(ctx, name, settings...); err != nil {
		t.Fatalf("ConfigureInterface(%s): %v", name, err)
	}
}

func peerUnit(units []PeerUnit, id UnitID) (PeerUnit, bool) {
	return find(units, func(u PeerUnit) bool { return u.ID == id })
}

// licensesExhausted reports whether all decoder licenses are in use.
func (d *device) licensesExhausted(t *testing.T) bool {
	t.Helper()
	h := d.nextStats(t).Health
	return h.DecoderLicenses > 0 && h.DecoderLicensesUsed >= h.DecoderLicenses
}

// tryUpdate is waitUpdate that reports a timeout instead of failing.
func (d *device) tryUpdate(from int, timeout time.Duration, ok func(Update) bool) (Update, bool) {
	deadline := time.After(timeout)
	for {
		d.mu.Lock()
		list, changed := d.updates[from:], d.changed
		d.mu.Unlock()
		for _, u := range list {
			if ok(u) {
				return u, true
			}
		}
		from += len(list)
		select {
		case <-changed:
		case <-deadline:
			return Update{}, false
		}
	}
}

// logUpdates logs the updates of an element after position from, to explain
// a failure.
func (d *device) logUpdates(t *testing.T, from int, id string) {
	t.Helper()
	d.mu.Lock()
	list := slices.Clone(d.updates[from:])
	d.mu.Unlock()
	for _, u := range list {
		switch {
		case u.ID != id:
		case u.Encoder != nil:
			e := u.Encoder
			t.Logf("  update %s: running=%t xlink=%t card=%v in=%s receiver=%s running=%t connected=%t",
				id, e.Running, e.XLink, e.Card, e.VideoIn, e.Receiver.ID, e.Receiver.Running, e.Receiver.Connected)
		case u.Decoder != nil:
			dec := u.Decoder
			t.Logf("  update %s: running=%t xlink=%t card=%v out=%s sender=%s running=%t connected=%t",
				id, dec.Running, dec.XLink, dec.Card, dec.VideoOut, dec.Sender.ID, dec.Sender.Running, dec.Sender.Connected)
		default:
			t.Logf("  update %s: %v removed=%t", id, u.Kind, u.Removed)
		}
	}
}

// isLinkUp reports whether the XLink connection of a local encoder is up and
// its receiver runs.
func (d *device) isLinkUp(t *testing.T, id UnitID) bool {
	e, _ := d.snapshot(t).Encoder(id)
	return e.XLink && e.Receiver.Connected && e.Receiver.Running
}

// waitCurrent returns the current state of an element if it already
// satisfies ok, or waits for an update after from that does.
func (d *device) waitCurrent(t *testing.T, from int, what string, current bool, ok func(Update) bool) {
	t.Helper()
	if current {
		return
	}
	d.waitUpdate(t, from, what, ok)
}

func TestDeviceChanges(t *testing.T) {
	d := connect(t)
	if d.peer == "" {
		t.Skip("XLINK_PEER is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Run("encoder settings", func(t *testing.T) {
		id := d.createUnit(t, ctx, UnitXLinkEncoder)
		// The configuration of the new unit is subscribed.
		d.waitUpdate(t, 0, "the configuration of "+string(id), encoderWhere(id, func(e *Encoder) bool { return e.Config != nil }))

		from := d.mark()
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
		// The settings arrive in systems.update and in the configuration;
		// every update carries the complete encoder.
		u := d.waitUpdate(t, from, "encoder settings", encoderWhere(id, func(e *Encoder) bool {
			return e.Name == "goxlinkclient test" && e.Bitrate == 25 && e.Config != nil &&
				e.Config.Bitrate == 25 && e.Config.MaxRTT == 300*time.Millisecond && e.Config.PacketAck
		}))
		if e := u.Encoder; e.Codec != VideoCodecH264 || e.FECLevel != 2 || e.Config.Name != "goxlinkclient test" {
			t.Errorf("encoder = codec %v fec %d config %+v", e.Codec, e.FECLevel, e.Config)
		}

		// H.265 limits the bitrate to 10 Mbps.
		from = d.mark()
		if err := d.c.ConfigureEncoder(ctx, id, EncoderCodec(VideoCodecH265)); err != nil {
			t.Fatalf("ConfigureEncoder(H.265): %v", err)
		}
		d.waitUpdate(t, from, "H.265 bitrate limit", encoderWhere(id, func(e *Encoder) bool {
			return e.Codec == VideoCodecH265 && e.Bitrate == 10 && e.Config != nil && e.Config.Bitrate == 10
		}))

		// Resetting the buffer of a stopped encoder is rejected.
		if err := d.c.ResetVideoBuffer(ctx, id); err == nil {
			t.Error("ResetVideoBuffer on a stopped encoder succeeded")
		} else {
			t.Logf("ResetVideoBuffer on stopped encoder: %v", err)
		}
	})

	t.Run("decoder linked to peer", func(t *testing.T) {
		id := d.createUnit(t, ctx, UnitXLinkDecoder)
		dec, _ := d.snapshot(t).Decoder(id)
		// Never start a decoder that would send a 2110 stream.
		if dec.Video2110.Enabled || dec.Audio2110.Enabled || dec.Card == VideoCardST2110 {
			t.Fatalf("new decoder has a 2110 output: %+v", dec)
		}

		// A decoder needs an output to run; the device keeps restarting one
		// without. SDI 3 is not used by the system.
		from := d.mark()
		err := d.c.ConfigureDecoder(ctx, id,
			DecoderName("goxlinkclient test"),
			DecoderCard(VideoCardSDI3),
			DecoderSignalGen(true),
			DecoderSignalGenDelay(30*time.Second),
			DecoderFPSSync(false),
		)
		if err != nil {
			t.Fatalf("ConfigureDecoder: %v", err)
		}
		d.waitUpdate(t, from, "decoder settings", decoderWhere(id, func(dec *Decoder) bool {
			return dec.Name == "goxlinkclient test" && dec.Card == VideoCardSDI3 && dec.SignalGenOn && dec.SignalGenDelay == 30*time.Second &&
				!dec.FPSSync && dec.Config != nil && !dec.Config.FPSSync && dec.Config.SignalGenDelay == 30*time.Second
		}))

		// Pick an idle, unlinked encoder of the peer.
		peer, ok := d.snapshot(t).Peer(d.peer)
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

		from = d.mark()
		if err := d.c.ConfigureDecoder(ctx, id, DecoderSender(sender.ID)); err != nil {
			t.Fatalf("ConfigureDecoder(sender): %v", err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			from := d.mark()
			if dec, _ := d.snapshot(t).Decoder(id); dec.Running {
				_ = d.c.Stop(ctx, id)
				d.waitUpdate(t, from, "stopped decoder", decoderWhere(id, func(dec *Decoder) bool { return !dec.Running }))
			}
			// Unlinking the decoder also clears the sender's receiver.
			if err := d.c.ConfigureDecoder(ctx, id, DecoderSender("")); err != nil {
				t.Errorf("unlink decoder: %v", err)
			}
			p, _ := d.snapshot(t).Peer(d.peer)
			u, _ := peerUnit(p.Encoders, sender.ID)
			d.waitCurrent(t, from, "sender restored", u.Linked == "" && !u.Running, peerWhere(d.peer, func(p *Peer) bool {
				u, ok := peerUnit(p.Encoders, sender.ID)
				return ok && u.Linked == "" && !u.Running
			}))
			t.Logf("restored %s", sender.ID)
		})
		// The link shows up on the decoder and on the peer; the decoder
		// receives the configuration of its sender.
		d.waitUpdate(t, from, "link", decoderWhere(id, func(dec *Decoder) bool { return dec.Sender.ID == sender.ID }))
		d.waitUpdate(t, from, "link on the peer", peerWhere(d.peer, func(p *Peer) bool {
			u, _ := peerUnit(p.Encoders, sender.ID)
			return u.Linked == id
		}))
		d.waitUpdate(t, from, "the configuration of the sender", decoderWhere(id, func(dec *Decoder) bool {
			return dec.Sender.ID == sender.ID && dec.Sender.Config != nil && dec.Sender.Config.Name == sender.Name
		}))
		// Units can only be started once the XLink connection is up.
		d.waitUpdate(t, from, "the XLink connection", decoderWhere(id, func(dec *Decoder) bool {
			return dec.XLink && dec.Sender.Connected
		}))

		// Only the decoder is started: the encoders of the peer take their
		// input from 2110 streams, and starting one would join multicast
		// groups on the peer's eth6.
		from = d.mark()
		if err := d.c.Start(ctx, id); err != nil {
			t.Fatalf("Start(%s): %v", id, err)
		}
		if u, running := d.tryUpdate(from, 30*time.Second, decoderWhere(id, func(dec *Decoder) bool {
			return dec.Running
		})); running {
			t.Logf("decoder running, sender running=%t connected=%t", u.Decoder.Sender.Running, u.Decoder.Sender.Connected)
		} else {
			// The device accepts start but does not run the decoder without a
			// free license.
			if d.licensesExhausted(t) {
				t.Skip("all decoder licenses are in use; the decoder cannot run")
			}
			d.logUpdates(t, from, string(id))
			t.Fatal("linked decoder did not start")
		}

		// The running decoder shows up in the statistics.
		timeout := time.After(20 * time.Second)
		for running := false; !running; {
			select {
			case u := <-d.stats:
				for _, ds := range u.Stats.Decoders {
					if ds.ID == id && ds.Running && ds.Sender.ID == sender.ID {
						t.Logf("stats: %.2f fps, %.2f Mbps, %d frames, sender running %t, RTT %v",
							ds.OutputFPS, ds.RX, ds.Video.Total, ds.Sender.Running, ds.Sender.XLink.RTT)
						running = true
					}
				}
			case <-timeout:
				t.Fatal("running decoder not in statistics")
			}
		}

		from = d.mark()
		if err := d.c.Stop(ctx, id); err != nil {
			t.Errorf("Stop(%s): %v", id, err)
		}
		d.waitUpdate(t, from, "stopped decoder", decoderWhere(id, func(dec *Decoder) bool { return !dec.Running }))
	})

	t.Run("encoder linked to peer decoder", func(t *testing.T) {
		id := d.createUnit(t, ctx, UnitXLinkEncoder)
		// A new encoder has no input selected; give it an SDI input that
		// sends bars without signal.
		from := d.mark()
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
		d.waitUpdate(t, from, "encoder input", encoderWhere(id, func(e *Encoder) bool {
			return e.Card == VideoCardSDI1 && e.Config != nil && e.Config.NoSignal == NoSignalBars
		}))

		// Pick an idle, unlinked decoder of the peer.
		peer, _ := d.snapshot(t).Peer(d.peer)
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

		// Link it: links are made on the decoder, here a decoder of the peer.
		// Its configuration then arrives with the encoder. The receiver was
		// unlinked before, which is restored at the end.
		from = d.mark()
		if err := d.c.ConfigureDecoder(ctx, receiver.ID, DecoderSender(id)); err != nil {
			t.Fatalf("ConfigureDecoder(%s, sender): %v", receiver.ID, err)
		}
		linked := d.waitUpdate(t, from, "the configuration of the receiver", encoderWhere(id, func(e *Encoder) bool {
			return e.Receiver.ID == receiver.ID && e.Receiver.Config != nil && e.Receiver.Config.Sender == id
		})).Encoder
		orig := *linked.Receiver.Config
		t.Logf("receiver config: card=%v video=%+v audio=%+v", orig.Card, orig.Video2110, orig.Audio2110)

		// Switch the receiver's outgoing 2110 streams off for the test; they
		// are restored exactly afterwards.
		if orig.Video2110.Enabled || orig.Audio2110.Enabled {
			offVideo, offAudio := orig.Video2110, orig.Audio2110
			offVideo.Enabled, offAudio.Enabled = false, false
			from := d.mark()
			if err := d.c.ConfigureDecoder(ctx, receiver.ID, DecoderVideo2110(offVideo), DecoderAudio2110(offAudio)); err != nil {
				t.Fatalf("disable 2110 output of %s: %v", receiver.ID, err)
			}
			d.waitUpdate(t, from, "2110 output off", encoderWhere(id, func(e *Encoder) bool {
				c := e.Receiver.Config
				return c != nil && !c.Video2110.Enabled && !c.Audio2110.Enabled
			}))
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			// Stop the units if the test did not get to it.
			from := d.mark()
			if e, _ := d.snapshot(t).Encoder(id); e.Running || e.Receiver.Running {
				_ = d.c.Stop(ctx, id)
				_ = d.c.Stop(ctx, receiver.ID)
				d.waitUpdate(t, from, "stopped link", encoderWhere(id, func(e *Encoder) bool {
					return !e.Running && !e.Receiver.Running
				}))
			}
			// Restore the receiver's 2110 output, then unlink it, which also
			// unlinks the encoder. The configuration of the receiver is no
			// longer subscribed afterwards, so it is checked first.
			from = d.mark()
			if err := d.c.ConfigureDecoder(ctx, receiver.ID, DecoderVideo2110(orig.Video2110), DecoderAudio2110(orig.Audio2110)); err != nil {
				t.Errorf("restore 2110 output of %s: %v", receiver.ID, err)
				return
			}
			if orig.Video2110.Enabled || orig.Audio2110.Enabled {
				d.waitUpdate(t, from, "2110 output restored", encoderWhere(id, func(e *Encoder) bool {
					c := e.Receiver.Config
					return c != nil && c.Video2110 == orig.Video2110 && c.Audio2110 == orig.Audio2110
				}))
			}
			from = d.mark()
			if err := d.c.ConfigureDecoder(ctx, receiver.ID, DecoderSender("")); err != nil {
				t.Errorf("unlink %s: %v", receiver.ID, err)
				return
			}
			d.waitUpdate(t, from, "receiver unlinked", encoderWhere(id, func(e *Encoder) bool { return e.Receiver.ID == "" }))
			d.waitUpdate(t, from, "receiver idle on the peer", peerWhere(d.peer, func(p *Peer) bool {
				u, ok := peerUnit(p.Decoders, receiver.ID)
				return ok && u.Linked == "" && !u.Running
			}))
			t.Logf("restored %s: unlinked, video %+v audio %+v", receiver.ID, orig.Video2110, orig.Audio2110)
		})

		// Units can only be started once the XLink connection is up (the
		// green dot in the web UI), which takes a few seconds.
		d.waitUpdate(t, from, "the XLink connection", encoderWhere(id, func(e *Encoder) bool {
			return e.XLink && e.Receiver.Connected
		}))

		// Starting the receiver reconnects the XLink tunnel; a start of the
		// encoder while it is down has no effect. The encoder is therefore
		// started once the receiver runs, and again if the tunnel was
		// reconnected in between.
		from = d.mark()
		if err := d.c.Start(ctx, receiver.ID); err != nil {
			t.Fatalf("Start(%s): %v", receiver.ID, err)
		}
		linkUp := encoderWhere(id, func(e *Encoder) bool { return e.XLink && e.Receiver.Connected && e.Receiver.Running })
		d.waitUpdate(t, from, "the running receiver", linkUp)
		running := false
		for attempt := 1; attempt <= 3 && !running; attempt++ {
			start := d.mark()
			if err := d.c.Start(ctx, id); err != nil {
				t.Fatalf("Start(%s): %v", id, err)
			}
			_, running = d.tryUpdate(start, 15*time.Second, encoderWhere(id, func(e *Encoder) bool {
				return e.Running && e.Receiver.Running
			}))
			if !running {
				t.Logf("start %d of %s had no effect", attempt, id)
				d.logUpdates(t, start, string(id))
				d.waitCurrent(t, start, "the XLink connection", d.isLinkUp(t, id), linkUp)
			}
		}
		if !running {
			d.logUpdates(t, from, string(id))
			e, _ := d.snapshot(t).Encoder(id)
			t.Fatalf("link did not start: encoder running=%t xlink=%t; receiver running=%t connected=%t",
				e.Running, e.XLink, e.Receiver.Running, e.Receiver.Connected)
		}

		timeout := time.After(20 * time.Second)
		for seen := false; !seen; {
			select {
			case u := <-d.stats:
				for _, es := range u.Stats.Encoders {
					if es.ID == id && es.Running && es.Receiver.ID == receiver.ID {
						t.Logf("stats: encoder %.2f fps, RTT %v; receiver running %t, %.2f fps, %.2f Mbps, %d frames",
							es.InputFPS, es.XLink.RTT, es.Receiver.Running, es.Receiver.OutputFPS, es.Receiver.RX, es.Receiver.Video.Total)
						seen = true
					}
				}
			case <-timeout:
				t.Fatal("running encoder not in statistics")
			}
		}

		from = d.mark()
		if err := d.c.Stop(ctx, id); err != nil {
			t.Errorf("Stop(%s): %v", id, err)
		}
		if err := d.c.Stop(ctx, receiver.ID); err != nil {
			t.Errorf("Stop(%s): %v", receiver.ID, err)
		}
		d.waitUpdate(t, from, "stopped link", encoderWhere(id, func(e *Encoder) bool {
			return !e.Running && !e.Receiver.Running
		}))
	})

	t.Run("SRT and NDI units", func(t *testing.T) {
		srtE := d.createUnit(t, ctx, UnitSRTEncoder)
		srtD := d.createUnit(t, ctx, UnitSRTDecoder)
		ndiE := d.createUnit(t, ctx, UnitNDIEncoder)
		ndiD := d.createUnit(t, ctx, UnitNDIDecoder)

		from := d.mark()
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
		d.waitUpdate(t, from, "SRT encoder settings", func(u Update) bool {
			e := u.SRTEncoder
			return u.Kind == UpdateSRTEncoder && u.ID == string(srtE) && e != nil && e.Name == "goxlinkclient srt" &&
				e.Bitrate == 8 && e.SRT == SRTConnection{Mode: SRTCaller, Address: "192.0.2.1", Port: 7981, LocalPort: 7982} &&
				e.Config != nil && e.Config.Mode == SRTCaller && e.Config.Address == "192.0.2.1" && e.Config.Port == 7981
		})

		from = d.mark()
		if err := d.c.ConfigureSRTDecoder(ctx, srtD, SRTDecoderName("goxlinkclient srt rx"), SRTDecoderMode(SRTListener)); err != nil {
			t.Fatalf("ConfigureSRTDecoder: %v", err)
		}
		d.waitUpdate(t, from, "SRT decoder settings", func(u Update) bool {
			dec := u.SRTDecoder
			return u.Kind == UpdateSRTDecoder && u.ID == string(srtD) && dec != nil && dec.Name == "goxlinkclient srt rx" &&
				dec.SRT.Mode == SRTListener && dec.Config != nil && dec.Config.Mode == SRTListener
		})

		from = d.mark()
		if err := d.c.ConfigureNDIEncoder(ctx, ndiE, NDIEncoderName("goxlinkclient ndi")); err != nil {
			t.Fatalf("ConfigureNDIEncoder: %v", err)
		}
		if err := d.c.ConfigureNDIDecoder(ctx, ndiD, NDIDecoderName("goxlinkclient ndi rx"), NDIDecoderFPSSync(true)); err != nil {
			t.Fatalf("ConfigureNDIDecoder: %v", err)
		}
		d.waitUpdate(t, from, "NDI encoder settings", func(u Update) bool {
			e := u.NDIEncoder
			return u.Kind == UpdateNDIEncoder && u.ID == string(ndiE) && e != nil && e.Name == "goxlinkclient ndi" &&
				e.Config != nil && e.Config.Name == "goxlinkclient ndi"
		})
		d.waitUpdate(t, from, "NDI decoder settings", func(u Update) bool {
			dec := u.NDIDecoder
			return u.Kind == UpdateNDIDecoder && u.ID == string(ndiD) && dec != nil && dec.Name == "goxlinkclient ndi rx" &&
				dec.FPSSync && dec.Config != nil && dec.Config.FPSSync
		})
	})

	t.Run("trunk", func(t *testing.T) {
		before := d.snapshot(t)
		from := d.mark()
		if err := d.c.CreateTrunk(ctx, before.ID); err != nil {
			t.Fatalf("CreateTrunk: %v", err)
		}
		u := d.waitUpdate(t, from, "new trunk", func(u Update) bool {
			if u.Kind != UpdateTrunk || u.Removed {
				return false
			}
			_, existed := before.Trunk(TrunkID(u.ID))
			return !existed
		})
		id := u.Trunk.ID
		t.Logf("created trunk %s on %q running=%t", id, u.Trunk.Interface, u.Trunk.Running)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			from := d.mark()
			if err := d.c.DeleteTrunk(ctx, id); err != nil {
				t.Errorf("DeleteTrunk(%s): %v", id, err)
				return
			}
			d.waitUpdate(t, from, "trunk deletion", removed(UpdateTrunk, string(id)))
			t.Logf("deleted trunk %s", id)
		})
		if u.Trunk.Running {
			t.Fatalf("new trunk %s is running", id)
		}

		from = d.mark()
		if err := d.c.ConfigureTrunk(ctx, id, TrunkMTU(1400), TrunkAutoStart(false)); err != nil {
			t.Fatalf("ConfigureTrunk: %v", err)
		}
		d.waitUpdate(t, from, "trunk MTU", func(u Update) bool {
			return u.Kind == UpdateTrunk && u.ID == string(id) && u.Trunk != nil && u.Trunk.MTU == 1400 && !u.Trunk.AutoStart
		})
	})

	t.Run("interface flags", func(t *testing.T) {
		// Use a disabled interface without link and address, which carries
		// no traffic.
		var orig Interface
		for _, i := range d.snapshot(t).Interfaces {
			if !slices.Contains(protectedInterfaces, i.ID) && !i.Enabled && !i.LinkUp && (i.IP == "" || i.IP == "0.0.0.0") {
				orig = i
				break
			}
		}
		if orig.ID == "" {
			t.Skip("no disabled interface without link and address")
		}
		name := orig.ID
		t.Logf("using %s", name)
		ifaceWhere := func(ok func(*Interface) bool) func(Update) bool {
			return func(u Update) bool {
				return u.Kind == UpdateInterface && u.ID == name && u.Interface != nil && ok(u.Interface)
			}
		}

		// The device rejects changes to disabled interfaces ("eth error"), so
		// the interface is enabled for the test.
		from := d.mark()
		d.configureInterface(t, ctx, name, InterfaceEnabled(true))
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			from := d.mark()
			d.configureInterface(t, ctx, name, InterfaceIGMP(orig.IGMP), InterfaceNMOS(orig.NMOS))
			d.configureInterface(t, ctx, name, InterfaceEnabled(false))
			d.waitUpdate(t, from, name+" restored", ifaceWhere(func(i *Interface) bool {
				return !i.Enabled && i.IGMP == orig.IGMP && i.NMOS == orig.NMOS
			}))
			if now, _ := d.snapshot(t).Interface(name); now != orig {
				t.Errorf("%s not restored:\nnow  %+v\nwant %+v", name, now, orig)
			}
			t.Logf("restored %s", name)
		})
		d.waitUpdate(t, from, name+" enabled", ifaceWhere(func(i *Interface) bool { return i.Enabled }))

		// Both flags are sent in one configEth request.
		from = d.mark()
		d.configureInterface(t, ctx, name, InterfaceIGMP(!orig.IGMP), InterfaceNMOS(!orig.NMOS))
		d.waitUpdate(t, from, name+" flags", ifaceWhere(func(i *Interface) bool {
			return i.IGMP == !orig.IGMP && i.NMOS == !orig.NMOS
		}))
	})
}

func TestDeviceRecon(t *testing.T) {
	d := connect(t)
	sys := d.snapshot(t)

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

	// Configurations of the local units arrive after the snapshot.
	time.Sleep(10 * time.Second)
	configs := 0
	for _, e := range d.snapshot(t).Encoders {
		if e.Config != nil {
			configs++
		}
	}
	t.Logf("encoders with configuration after 10 s: %d of %d", configs, len(sys.Encoders))

	s := d.nextStats(t)
	t.Logf("stats: health %+v", s.Health)
	t.Logf("stats: %d interfaces, %d enc, %d dec, %d peers", len(s.Interfaces), len(s.Encoders), len(s.Decoders), len(s.Peers))
}
