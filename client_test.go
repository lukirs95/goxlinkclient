package xlinkclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const fakeSystemID = "X8A1001"

// request is a request as received by the fake device.
type request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	ID     int             `json:"id"`
}

// fakeDevice is a minimal VideoXLink websocket endpoint: it announces itself,
// checks the password, sends systems.full and answers every other request.
type fakeDevice struct {
	t           *testing.T
	srv         *httptest.Server
	password    string
	full        []byte
	systemStats []byte
	localStats  []byte
	reject      map[string]bool
	requests    chan request
	// statsSubscribe receives the params of localStats.subscribe.
	statsSubscribe chan json.RawMessage
}

func newFakeDevice(t *testing.T, password string) *fakeDevice {
	t.Helper()
	d := &fakeDevice{
		t:              t,
		password:       password,
		full:           readFixture(t, "systems_full_1.8.json"),
		systemStats:    readFixture(t, "stats_system_1.8.json"),
		localStats:     readFixture(t, "stats_local_1.8.json"),
		reject:         map[string]bool{},
		requests:       make(chan request, 16),
		statsSubscribe: make(chan json.RawMessage, 1),
	}
	d.srv = httptest.NewServer(http.HandlerFunc(d.serve))
	t.Cleanup(d.srv.Close)
	return d
}

// addr returns host:port of the fake device.
func (d *fakeDevice) addr() string {
	return strings.TrimPrefix(d.srv.URL, "http://")
}

func (d *fakeDevice) serve(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	ctx := r.Context()

	notify := func(method string, params json.RawMessage) error {
		return wsjson.Write(ctx, conn, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	}
	respond := func(id int, result any) error {
		return wsjson.Write(ctx, conn, map[string]any{"jsonrpc": "2.0", "result": result, "id": id})
	}

	if err := notify("notify.auth", json.RawMessage(`{"sysid":"`+fakeSystemID+`"}`)); err != nil {
		return
	}
	for {
		var req request
		if err := wsjson.Read(ctx, conn, &req); err != nil {
			return
		}
		if req.Method == "auth" {
			var p authParams
			_ = json.Unmarshal(req.Params, &p)
			ok := p.Password == d.password
			if err := respond(req.ID, map[string]any{"method": "auth", "response": ok}); err != nil || !ok {
				return
			}
			// The device announces itself again after login; the client must
			// not stall on it.
			if err := notify("notify.auth", json.RawMessage(`{"sysid":"`+fakeSystemID+`"}`)); err != nil {
				return
			}
			if err := notify("systems.full", d.full); err != nil {
				return
			}
			continue
		}
		if req.Method == "localStats.subscribe" {
			select {
			case d.statsSubscribe <- req.Params:
			default:
			}
			if err := respond(req.ID, map[string]any{"method": req.Method, "response": true}); err != nil {
				return
			}
			// The history is not subscribed by the client and must be
			// ignored; health arrives before the unit statistics.
			if err := notify("systems.localStatsHistory", json.RawMessage(`{"sysid":"`+fakeSystemID+`","data":[]}`)); err != nil {
				return
			}
			if err := notify("systems.stats", d.systemStats); err != nil {
				return
			}
			if err := notify("systems.localStats", d.localStats); err != nil {
				return
			}
			continue
		}
		d.requests <- req
		if err := respond(req.ID, map[string]any{"method": req.Method, "response": !d.reject[req.Method]}); err != nil {
			return
		}
	}
}

// nextRequest returns the next request received by the fake device.
func (d *fakeDevice) nextRequest(t *testing.T) request {
	t.Helper()
	select {
	case req := <-d.requests:
		return req
	case <-time.After(2 * time.Second):
		t.Fatal("no request received")
		return request{}
	}
}

// startClient runs a client against d and returns it with the channel of its
// updates and a function that stops it and returns the result of Run.
func startClient(t *testing.T, d *fakeDevice, password string) (*Client, chan Update, func() error) {
	t.Helper()
	updates := make(chan Update)
	c := New(d.addr(), WithCredentials("admin", password), WithUpdates(updates))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	stop := func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not return")
			return nil
		}
	}
	t.Cleanup(func() { cancel() })
	return c, updates, stop
}

func TestRunRequiresCredentials(t *testing.T) {
	if err := New("127.0.0.1:1").Run(context.Background()); !errors.Is(err, ErrNoCredentials) {
		t.Errorf("Run without credentials = %v, want ErrNoCredentials", err)
	}
}

func TestRunAuthFailed(t *testing.T) {
	d := newFakeDevice(t, "secret")
	c := New(d.addr(), WithCredentials("admin", "wrong"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Run(ctx); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("Run with wrong password = %v, want ErrAuthFailed", err)
	}
	if c.Ready() {
		t.Error("Ready() = true after failed authentication")
	}
}

func TestClientEndToEnd(t *testing.T) {
	d := newFakeDevice(t, "secret")
	c, updates, stop := startClient(t, d, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.WaitReady(ctx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	if c.SystemID() != fakeSystemID {
		t.Errorf("SystemID() = %q, want %q", c.SystemID(), fakeSystemID)
	}

	select {
	case u := <-updates:
		if u.Client != c || u.System.ID != fakeSystemID || len(u.System.Encoders) != 5 {
			t.Errorf("update = client %p system %s with %d encoders", u.Client, u.System.ID, len(u.System.Encoders))
		}
	case <-ctx.Done():
		t.Fatal("no update received")
	}
	if sys, ok := c.Snapshot(); !ok || sys.ID != fakeSystemID {
		t.Errorf("Snapshot() = %s, %t", sys.ID, ok)
	}

	// All encoder settings go out in one config request, addressed via the
	// local system even for a remote unit.
	err := c.ConfigureEncoder(ctx, "X8A2222-E5",
		EncoderBitrate(25),
		EncoderVideo2110(Stream{Interface: "eth6", Enabled: true, Address: "239.0.0.1", Port: 30000}),
	)
	if err != nil {
		t.Fatalf("ConfigureEncoder: %v", err)
	}
	req := d.nextRequest(t)
	var params struct {
		SysID  string         `json:"sysid"`
		ID     string         `json:"id"`
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatal(err)
	}
	if req.Method != "config" || params.SysID != fakeSystemID || params.ID != "X8A2222-E5" {
		t.Errorf("request = %s sysid=%s id=%s", req.Method, params.SysID, params.ID)
	}
	if params.Values["vTBR"] != float64(25) || params.Values["v2110NetPriPort"] != "30000" || len(params.Values) != 5 {
		t.Errorf("values = %v", params.Values)
	}

	// A rejected request becomes a DeviceError.
	d.reject["stop"] = true
	err = c.Stop(ctx, "X8A1001-D1")
	var devErr *DeviceError
	if !errors.As(err, &devErr) || devErr.Method != "stop" {
		t.Errorf("Stop = %v, want DeviceError for stop", err)
	}
	if req := d.nextRequest(t); req.Method != "stop" || !strings.Contains(string(req.Params), `"id":"X8A1001-D1"`) {
		t.Errorf("request = %s %s", req.Method, req.Params)
	}

	// A trunk is created on the given system, which may be a remote one.
	if err := c.CreateTrunk(ctx, "X8A2222"); err != nil {
		t.Fatalf("CreateTrunk: %v", err)
	}
	if req := d.nextRequest(t); req.Method != "newL2S" || string(req.Params) != `{"sysid":"X8A2222"}` {
		t.Errorf("request = %s %s", req.Method, req.Params)
	}

	// PTP settings go out one request each, in order.
	if err := c.ConfigurePTP(ctx, PTPEnabled(true), PTPDomain(99)); err != nil {
		t.Fatalf("ConfigurePTP: %v", err)
	}
	for _, want := range []string{`{"ptp":true}`, `{"ptpDomainNumber":"99"}`} {
		req := d.nextRequest(t)
		if req.Method != "set2110" || !strings.Contains(string(req.Params), want) {
			t.Errorf("request = %s %s, want values %s", req.Method, req.Params, want)
		}
	}

	if err := stop(); err != nil {
		t.Errorf("Run returned %v after cancel, want nil", err)
	}
	if c.Ready() {
		t.Error("Ready() = true after Run returned")
	}
	if err := c.Start(context.Background(), "X8A1001-E1"); !errors.Is(err, ErrNotConnected) {
		t.Errorf("Start after Run = %v, want ErrNotConnected", err)
	}
}

func TestStatsEndToEnd(t *testing.T) {
	d := newFakeDevice(t, "secret")
	stats := make(chan StatsUpdate)
	c := New(d.addr(), WithCredentials("admin", "secret"), WithStats(stats))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	select {
	case params := <-d.statsSubscribe:
		// A minimal history keeps messages far below the read limit.
		if string(params) != `{"batch":1,"max":1,"sysid":"local"}` {
			t.Errorf("localStats.subscribe params = %s", params)
		}
	case <-ctx.Done():
		t.Fatal("localStats.subscribe not received")
	}

	select {
	case u := <-stats:
		s := u.Stats
		if u.Client != c || s.System != fakeSystemID {
			t.Errorf("stats from client %p system %s", u.Client, s.System)
		}
		// Units come from systems.localStats, health from systems.stats.
		if len(s.Decoders) != 5 || len(s.Encoders) != 5 || len(s.Interfaces) != 4 {
			t.Errorf("counts enc=%d dec=%d nets=%d", len(s.Encoders), len(s.Decoders), len(s.Interfaces))
		}
		if s.Health.CPUTemp != 63 || !s.Health.PTPSync || len(s.Peers) != 1 {
			t.Errorf("Health = %+v, peers %d", s.Health, len(s.Peers))
		}
	case <-ctx.Done():
		t.Fatal("no statistics received")
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run returned %v", err)
	}
}

func TestRunNotConcurrent(t *testing.T) {
	d := newFakeDevice(t, "secret")
	c, _, stop := startClient(t, d, "secret")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.WaitReady(ctx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	if err := c.Run(ctx); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Run = %v, want ErrAlreadyRunning", err)
	}
	_ = stop()
}
