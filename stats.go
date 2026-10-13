package xlinkclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// The device sends statistics in two messages. systems.localStats carries the
// local XLink units and the active interfaces together with their
// counterparts; the web UI plots it. systems.stats carries every unit of the
// local and the remote systems and the system health; the web UI shows it as
// text. Stats contains the values the web UI shows: those of
// systems.localStats, completed with the values only systems.stats reports,
// matched by unit ID.

// Stats combines the unit and interface statistics of systems.localStats with
// the most recent systems.stats message. A Stats value is delivered for every
// systems.localStats message. Values only systems.stats reports are zero until
// its first message has been received.
type Stats struct {
	System SystemID
	Time   time.Time
	// Health is the most recent system health.
	Health     Health
	Interfaces []InterfaceStats
	Encoders   []EncoderStats
	Decoders   []DecoderStats
	// Peers are the remote systems reported by systems.stats.
	Peers []PeerStats
}

// Health is the state of a system's hardware and services.
type Health struct {
	PTP     bool
	PTPSync bool
	NMOS    bool
	Uptime  time.Duration
	// CPU is the CPU load in percent.
	CPU int
	// CPUTemp is the CPU temperature in degrees Celsius.
	CPUTemp             int
	Licenses            int
	LicensesUsed        int
	DecoderLicenses     int
	DecoderLicensesUsed int
}

// PeerStats is the health of a remote system.
type PeerStats struct {
	ID     SystemID
	Health Health
	// RTT is the round trip time to the remote system.
	RTT time.Duration
}

// InterfaceStats is the throughput of a network interface.
type InterfaceStats struct {
	ID string
	// RX and TX are in Mbps.
	RX float64
	TX float64
}

// EncoderXLinkStats are the transport statistics of an XLink encoder.
type EncoderXLinkStats struct {
	RTT time.Duration
	// ACK reports whether the receiver acknowledges packets and Resend
	// whether unacknowledged packets are resent. As in the web UI, Resend is
	// only reported while ACK is on.
	ACK    bool
	Resend bool
	Cloud  int64
	Resent int64
	NoACK  int64
	// LateACK and DroppedACK count acknowledgements that arrived late or not
	// at all (web UI "Late ACK").
	LateACK    int64
	DroppedACK int64
	// ResentACK, ResentLateACK and ResentDroppedACK count the
	// acknowledgements of resent packets (web UI "Resent ACK": First, Late,
	// Drop).
	ResentACK        int64
	ResentLateACK    int64
	ResentDroppedACK int64
}

// DecoderXLinkStats are the transport statistics of an XLink decoder.
type DecoderXLinkStats struct {
	Resent     int64
	ResentDrop int64
	Drop       int64
}

// AudioLevel is the level of an audio channel in dBFS.
type AudioLevel struct {
	Channel int
	Level   float64
	Peak    float64
}

// EncoderMetrics are the statistics reported for an XLink encoder, either for
// a local encoder or as the sender of a local decoder.
type EncoderMetrics struct {
	ID      UnitID
	Running bool
	Uptime  time.Duration
	// StatsTime is the time the counters have been collected for.
	StatsTime time.Duration
	InputFPS  float64
	// TTF is shown as "TTF" by the web UI.
	TTF   time.Duration
	XLink EncoderXLinkStats
	Audio []AudioLevel
}

// EncoderStats are the statistics of a local XLink encoder and its receiver.
type EncoderStats struct {
	EncoderMetrics
	// Receiver is reported by the decoder this encoder sends to. Its ID is
	// empty if no receiver is selected.
	Receiver DecoderMetrics
}

// DecoderMetrics are the statistics reported for an XLink decoder, either for
// a local decoder or as the receiver of a local encoder.
type DecoderMetrics struct {
	ID      UnitID
	Running bool
	Uptime  time.Duration
	// StatsTime is the time the counters have been collected for.
	StatsTime time.Duration
	// DecodeFPS is the frame rate of the decoded input, OutputFPS that of the
	// output.
	DecodeFPS float64
	OutputFPS float64
	// RX is the received bitrate in Mbps.
	RX      float64
	XLink   DecoderXLinkStats
	Video   VideoStats
	Audio   AudioStats
	Packets PacketStats
	Buffer  BufferStats
}

// DecoderStats are the statistics of a local XLink decoder and its sender.
type DecoderStats struct {
	DecoderMetrics
	// Sender is reported by the encoder this decoder receives from. Its ID is
	// empty if no sender is selected.
	Sender EncoderMetrics
}

// VideoStats count decoded video frames. The web UI shows Total as "Frames
// Tot" and the FEC results as "Tot Ok", "Corrected" and "Error".
type VideoStats struct {
	Total        int64
	Dropped      int64
	Missing      int64
	FECOK        int64
	FECCorrected int64
	FECFailed    int64
}

// AudioStats count decoded audio frames.
type AudioStats struct {
	Dropped  int64
	Missing  int64
	Unsynced int64
}

// PacketStats count received packets that were late or missing. Each count
// is reported in total and for the last interval.
type PacketStats struct {
	Late            int64
	LateLast        int64
	Missing         int64
	MissingLast     int64
	LateDropped     int64
	LateDroppedLast int64
	// SlowDecode counts video errors caused by too slow decoding.
	SlowDecode int64
}

// BufferStats describe the playout buffer.
type BufferStats struct {
	// Delay is the configured playout delay and HeadMin the smallest headroom
	// left in the buffer; the web UI plots (Delay-HeadMin)/Delay as buffer
	// usage.
	Delay   time.Duration
	HeadMin time.Duration
	// Warning is set if the buffer is too small for the ARQ settings; Min is
	// the smallest sufficient buffer.
	Warning bool
	Min     time.Duration
}

// Stats item types used by both statistics messages.
const (
	statsTypeSystem  = 0 // the system itself and its interfaces
	statsTypeEncoder = 1
	statsTypeDecoder = 2
)

// errBadStats is wrapped by errors for statistics messages that cannot be read
// at all.
var errBadStats = errors.New("xlinkclient: malformed statistics message")

func millisDuration(ms float64) time.Duration {
	return time.Duration(ms * float64(time.Millisecond))
}

func secondsDuration(s int) time.Duration {
	return time.Duration(s) * time.Second
}

type wireStatsItem struct {
	ID   string          `json:"id"`
	Type int             `json:"type"`
	Data json.RawMessage `json:"data"`
}

type wireInterfaceStats struct {
	RX flexFloat `json:"rx"`
	TX flexFloat `json:"tx"`
}

// wireXLinkStats holds the keys of encoders and decoders alike.
type wireXLinkStats struct {
	RTT           flexFloat `json:"rtt"`
	XAck          flexBool  `json:"xAck"`
	XAckR         flexBool  `json:"xAckR"`
	Cloud         flexInt   `json:"cloud"`
	Resent        flexInt   `json:"resent"`
	ResentDrop    flexInt   `json:"resentDrop"`
	Drop          flexInt   `json:"drop"`
	NoAck         flexInt   `json:"noAck"`
	LateAck       flexInt   `json:"lateAck"`
	DropAck       flexInt   `json:"dropAck"`
	ResentAck     flexInt   `json:"resentAck"`
	LateAckResent flexInt   `json:"lateAckResent"`
	DropAckResent flexInt   `json:"dropAckResent"`
}

// wireAudioLevel is an element of "aInCh".
type wireAudioLevel struct {
	ID   flexInt   `json:"id"`
	RMS  flexFloat `json:"dBFS_RMS"`
	Peak flexFloat `json:"dBFS_peak"`
}

// flexObject is a JSON object whose keys are not used, e.g. the audio
// channels. An array is accepted as well, as an empty
// collection might be encoded as [].
type flexObject[T any] []T

// UnmarshalJSON implements json.Unmarshaler. Elements are ordered by key.
func (o *flexObject[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.HasPrefix(data, []byte("[")) {
		var items []T
		err := json.Unmarshal(data, &items)
		*o = items
		return err
	}
	var m map[string]T
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*o = make([]T, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		*o = append(*o, m[k])
	}
	return nil
}

type wireEncoderMetrics struct {
	ID        string                     `json:"id"`
	Running   flexBool                   `json:"running"`
	UpTime    flexInt                    `json:"upTime"`
	StatsTime flexInt                    `json:"statsTime"`
	VInFps    flexFloat                  `json:"vInFps"`
	VInT      flexInt                    `json:"vInT"`
	XLink     wireXLinkStats             `json:"xLink"`
	AInCh     flexObject[wireAudioLevel] `json:"aInCh"`
}

// local converts the values systems.localStats reports for an encoder.
func (m *wireEncoderMetrics) local(c *collector, id string) EncoderMetrics {
	if m.ID != "" {
		id = m.ID
	}
	x := &m.XLink
	xc := c.scope("xLink")
	e := EncoderMetrics{
		ID:       unitID(id),
		Running:  c.bool("running", &m.Running),
		Uptime:   secondsDuration(c.int("upTime", &m.UpTime)),
		InputFPS: c.float("vInFps", &m.VInFps),
		TTF:      time.Duration(c.int("vInT", &m.VInT)) * time.Millisecond,
		XLink: EncoderXLinkStats{
			RTT:    millisDuration(xc.float("rtt", &x.RTT)),
			Resent: int64(xc.int("resent", &x.Resent)),
		},
	}
	c.merge(xc)
	return e
}

// complete adds the values only systems.stats reports for an encoder. The
// sender of a decoder carries no uptime in systems.localStats.
func (m *wireEncoderMetrics) complete(c *collector, e *EncoderMetrics, counterpart bool) {
	if counterpart {
		e.Uptime = secondsDuration(c.int("upTime", &m.UpTime))
	}
	e.StatsTime = secondsDuration(c.int("statsTime", &m.StatsTime))

	x := &m.XLink
	xc := c.scope("xLink")
	ack := xc.bool("xAck", &x.XAck)
	resend := xc.bool("xAckR", &x.XAckR)
	e.XLink.ACK = ack
	e.XLink.Resend = ack && resend
	e.XLink.Cloud = int64(xc.int("cloud", &x.Cloud))
	e.XLink.NoACK = int64(xc.int("noAck", &x.NoAck))
	e.XLink.LateACK = int64(xc.int("lateAck", &x.LateAck))
	e.XLink.DroppedACK = int64(xc.int("dropAck", &x.DropAck))
	e.XLink.ResentACK = int64(xc.int("resentAck", &x.ResentAck))
	e.XLink.ResentLateACK = int64(xc.int("lateAckResent", &x.LateAckResent))
	e.XLink.ResentDroppedACK = int64(xc.int("dropAckResent", &x.DropAckResent))
	c.merge(xc)

	ac := c.scope("aInCh")
	e.Audio = make([]AudioLevel, 0, len(m.AInCh))
	for i := range m.AInCh {
		l := &m.AInCh[i]
		e.Audio = append(e.Audio, AudioLevel{
			Channel: ac.int("id", &l.ID),
			Level:   ac.float("dBFS_RMS", &l.RMS),
			Peak:    ac.float("dBFS_peak", &l.Peak),
		})
	}
	slices.SortFunc(e.Audio, func(a, b AudioLevel) int { return a.Channel - b.Channel })
	c.merge(ac)
}

type wireEncoderStats struct {
	wireEncoderMetrics
	Receiver wireDecoderMetrics `json:"receiver"`
}

type wireDecoderMetrics struct {
	ID        string         `json:"id"`
	Running   flexBool       `json:"running"`
	UpTime    flexInt        `json:"upTime"`
	StatsTime flexInt        `json:"statsTime"`
	VDecFps   flexFloat      `json:"vDecFps"`
	VOutFps   flexFloat      `json:"vOutFps"`
	Mbps      wireMbps       `json:"mbps"`
	XLink     wireXLinkStats `json:"xLink"`
	VDstats   struct {
		Total        flexInt `json:"total"`
		Drop         flexInt `json:"drop"`
		Missing      flexInt `json:"missing"`
		FECOK        flexInt `json:"fecok"`
		FECCorrected flexInt `json:"fecCorrected"`
		FECNOK       flexInt `json:"fecnok"`
	} `json:"vDstats"`
	ADstats struct {
		Drop   flexInt `json:"drop"`
		Miss   flexInt `json:"miss"`
		Unsync flexInt `json:"unsync"`
	} `json:"aDstats"`
	VPbuffer struct {
		Min     flexFloat `json:"min"`
		Warning flexBool  `json:"warning"`
	} `json:"vPbuffer"`
	// vRstats has different keys in both messages: the buffer values in
	// systems.localStats, the packet counts in systems.stats. "late" means
	// something else in each and is only read from systems.stats.
	VRstats struct {
		Delay        flexFloat `json:"delay"`
		HeadMin      flexFloat `json:"head_min_ms"`
		Late         flexInt   `json:"late"`
		LateLast     flexInt   `json:"lateLast"`
		Missing      flexInt   `json:"missing"`
		Last         flexInt   `json:"last"`
		LateDrop     flexInt   `json:"lateDrop"`
		LateDropLast flexInt   `json:"lateDropLast"`
		SlowDecode   flexInt   `json:"slowDecode"`
	} `json:"vRstats"`
}

type wireMbps struct {
	RX flexFloat `json:"rx"`
}

// local converts the values systems.localStats reports for a decoder.
func (m *wireDecoderMetrics) local(c *collector, id string) DecoderMetrics {
	if m.ID != "" {
		id = m.ID
	}
	x := &m.XLink
	xc := c.scope("xLink")
	d := DecoderMetrics{
		ID:        unitID(id),
		Running:   c.bool("running", &m.Running),
		Uptime:    secondsDuration(c.int("upTime", &m.UpTime)),
		OutputFPS: c.float("vOutFps", &m.VOutFps),
		RX:        c.float("mbps.rx", &m.Mbps.RX),
		XLink: DecoderXLinkStats{
			Resent:     int64(xc.int("resent", &x.Resent)),
			ResentDrop: int64(xc.int("resentDrop", &x.ResentDrop)),
			Drop:       int64(xc.int("drop", &x.Drop)),
		},
	}
	c.merge(xc)

	v := &m.VDstats
	d.Video = VideoStats{
		Dropped:      int64(c.int("vDstats.drop", &v.Drop)),
		Missing:      int64(c.int("vDstats.missing", &v.Missing)),
		FECCorrected: int64(c.int("vDstats.fecCorrected", &v.FECCorrected)),
		FECFailed:    int64(c.int("vDstats.fecnok", &v.FECNOK)),
	}
	a := &m.ADstats
	d.Audio = AudioStats{
		Dropped:  int64(c.int("aDstats.drop", &a.Drop)),
		Missing:  int64(c.int("aDstats.miss", &a.Miss)),
		Unsynced: int64(c.int("aDstats.unsync", &a.Unsync)),
	}
	d.Buffer = BufferStats{
		Delay:   millisDuration(c.float("vRstats.delay", &m.VRstats.Delay)),
		HeadMin: millisDuration(c.float("vRstats.head_min_ms", &m.VRstats.HeadMin)),
		Warning: c.bool("vPbuffer.warning", &m.VPbuffer.Warning),
		Min:     millisDuration(c.float("vPbuffer.min", &m.VPbuffer.Min)),
	}
	return d
}

// complete adds the values only systems.stats reports for a decoder. The
// receiver of an encoder carries no uptime in systems.localStats.
func (m *wireDecoderMetrics) complete(c *collector, d *DecoderMetrics, counterpart bool) {
	if counterpart {
		d.Uptime = secondsDuration(c.int("upTime", &m.UpTime))
	}
	d.StatsTime = secondsDuration(c.int("statsTime", &m.StatsTime))
	d.DecodeFPS = c.float("vDecFps", &m.VDecFps)
	d.Video.Total = int64(c.int("vDstats.total", &m.VDstats.Total))
	d.Video.FECOK = int64(c.int("vDstats.fecok", &m.VDstats.FECOK))
	r := &m.VRstats
	d.Packets = PacketStats{
		Late:            int64(c.int("vRstats.late", &r.Late)),
		LateLast:        int64(c.int("vRstats.lateLast", &r.LateLast)),
		Missing:         int64(c.int("vRstats.missing", &r.Missing)),
		MissingLast:     int64(c.int("vRstats.last", &r.Last)),
		LateDropped:     int64(c.int("vRstats.lateDrop", &r.LateDrop)),
		LateDroppedLast: int64(c.int("vRstats.lateDropLast", &r.LateDropLast)),
		SlowDecode:      int64(c.int("vRstats.slowDecode", &r.SlowDecode)),
	}
}

type wireDecoderStats struct {
	wireDecoderMetrics
	Sender wireEncoderMetrics `json:"sender"`
}

// Wire types of systems.localStats.

type wireLocalStats struct {
	SysID string          `json:"sysid"`
	Time  flexTime        `json:"time"`
	Data  []wireStatsItem `json:"data"`
}

// localStats is a decoded systems.localStats message.
type localStats struct {
	System     SystemID
	Time       time.Time
	Interfaces []InterfaceStats
	Encoders   []EncoderStats
	Decoders   []DecoderStats
}

// decodeLocalStats decodes a systems.localStats message. Items of other types,
// e.g. NDI units, are ignored.
func decodeLocalStats(params json.RawMessage) (localStats, []decodeIssue, error) {
	var w wireLocalStats
	if err := json.Unmarshal(params, &w); err != nil {
		return localStats{}, nil, fmt.Errorf("%w: %w", errBadStats, err)
	}

	var c collector
	ls := localStats{
		System:     SystemID(w.SysID),
		Time:       c.time("time", &w.Time),
		Interfaces: []InterfaceStats{},
		Encoders:   []EncoderStats{},
		Decoders:   []DecoderStats{},
	}
	for _, item := range w.Data {
		if len(item.Data) == 0 {
			continue
		}
		ic := c.scope(item.ID)
		switch {
		case item.Type == statsTypeSystem && strings.HasPrefix(item.ID, "eth"):
			var s wireInterfaceStats
			if err := json.Unmarshal(item.Data, &s); err != nil {
				ic.add(issueFromError("data", item.Data, err))
				break
			}
			ls.Interfaces = append(ls.Interfaces, InterfaceStats{
				ID: item.ID,
				RX: ic.float("rx", &s.RX),
				TX: ic.float("tx", &s.TX),
			})
		case item.Type == statsTypeEncoder:
			var s wireEncoderStats
			if err := json.Unmarshal(item.Data, &s); err != nil {
				ic.add(issueFromError("data", item.Data, err))
				break
			}
			rc := ic.scope("receiver")
			ls.Encoders = append(ls.Encoders, EncoderStats{
				EncoderMetrics: s.wireEncoderMetrics.local(ic, item.ID),
				Receiver:       s.Receiver.local(rc, ""),
			})
			ic.merge(rc)
		case item.Type == statsTypeDecoder:
			var s wireDecoderStats
			if err := json.Unmarshal(item.Data, &s); err != nil {
				ic.add(issueFromError("data", item.Data, err))
				break
			}
			sc := ic.scope("sender")
			ls.Decoders = append(ls.Decoders, DecoderStats{
				DecoderMetrics: s.wireDecoderMetrics.local(ic, item.ID),
				Sender:         s.Sender.local(sc, ""),
			})
			ic.merge(sc)
		}
		c.merge(ic)
	}
	return ls, c.issues, nil
}

// Wire types of systems.stats.

type wireSystemStats struct {
	SysID string `json:"sysid"`
	Data  struct {
		Local  []wireStatsItem `json:"local"`
		Remote []struct {
			SysID string          `json:"sysid"`
			Data  []wireStatsItem `json:"data"`
		} `json:"remote"`
	} `json:"data"`
}

type wireHealth struct {
	PTPSync     flexBool  `json:"ptpSync"`
	PTP         flexBool  `json:"ptp"`
	NMOS        flexBool  `json:"nmos"`
	OSUpTime    flexInt   `json:"osUpTime"`
	CPU         flexInt   `json:"cpu"`
	CPUTemp     flexInt   `json:"cpuTemp"`
	VLic        flexInt   `json:"vLic"`
	VLicDec     flexInt   `json:"vLicDec"`
	VLicUsed    flexInt   `json:"vLicUsed"`
	VLicUsedDec flexInt   `json:"vLicUsedDec"`
	RTT         flexFloat `json:"rtt"`
}

func (h *wireHealth) convert(c *collector) Health {
	return Health{
		PTP:                 c.bool("ptp", &h.PTP),
		PTPSync:             c.bool("ptpSync", &h.PTPSync),
		NMOS:                c.bool("nmos", &h.NMOS),
		Uptime:              secondsDuration(c.int("osUpTime", &h.OSUpTime)),
		CPU:                 c.int("cpu", &h.CPU),
		CPUTemp:             c.int("cpuTemp", &h.CPUTemp),
		Licenses:            c.int("vLic", &h.VLic),
		LicensesUsed:        c.int("vLicUsed", &h.VLicUsed),
		DecoderLicenses:     c.int("vLicDec", &h.VLicDec),
		DecoderLicensesUsed: c.int("vLicUsedDec", &h.VLicUsedDec),
	}
}

// findHealth returns the health item of system id in items.
func findHealth(c *collector, id string, items []wireStatsItem) (Health, float64, bool) {
	for _, item := range items {
		if item.ID != id || item.Type != statsTypeSystem || len(item.Data) == 0 {
			continue
		}
		var h wireHealth
		if err := json.Unmarshal(item.Data, &h); err != nil {
			c.add(issueFromError(id, item.Data, err))
			return Health{}, 0, false
		}
		hc := c.scope(id)
		health, rtt := h.convert(hc), hc.float("rtt", &h.RTT)
		c.merge(hc)
		return health, rtt, true
	}
	return Health{}, 0, false
}

// systemStats is a decoded systems.stats message. The XLink units of all
// systems are kept by ID to complete systems.localStats.
type systemStats struct {
	health   Health
	peers    []PeerStats
	encoders map[UnitID]*wireEncoderMetrics
	decoders map[UnitID]*wireDecoderMetrics
}

// decodeSystemStats decodes a systems.stats message.
func decodeSystemStats(params json.RawMessage) (systemStats, []decodeIssue, error) {
	var w wireSystemStats
	if err := json.Unmarshal(params, &w); err != nil {
		return systemStats{}, nil, fmt.Errorf("%w: %w", errBadStats, err)
	}

	var c collector
	ss := systemStats{
		peers:    make([]PeerStats, 0, len(w.Data.Remote)),
		encoders: map[UnitID]*wireEncoderMetrics{},
		decoders: map[UnitID]*wireDecoderMetrics{},
	}
	ss.health, _, _ = findHealth(&c, w.SysID, w.Data.Local)
	ss.addUnits(&c, w.Data.Local)
	for _, r := range w.Data.Remote {
		ss.addUnits(&c, r.Data)
		h, rtt, ok := findHealth(&c, r.SysID, r.Data)
		if !ok {
			continue
		}
		ss.peers = append(ss.peers, PeerStats{ID: SystemID(r.SysID), Health: h, RTT: millisDuration(rtt)})
	}
	return ss, c.issues, nil
}

// addUnits adds the XLink units of one system.
func (ss *systemStats) addUnits(c *collector, items []wireStatsItem) {
	for _, item := range items {
		if len(item.Data) == 0 {
			continue
		}
		ic := c.scope(item.ID)
		switch item.Type {
		case statsTypeEncoder:
			var m wireEncoderMetrics
			if err := json.Unmarshal(item.Data, &m); err != nil {
				ic.add(issueFromError("data", item.Data, err))
				break
			}
			ss.encoders[UnitID(item.ID)] = &m
		case statsTypeDecoder:
			var m wireDecoderMetrics
			if err := json.Unmarshal(item.Data, &m); err != nil {
				ic.add(issueFromError("data", item.Data, err))
				break
			}
			ss.decoders[UnitID(item.ID)] = &m
		}
		c.merge(ic)
	}
}

// mergeStats completes the local statistics with the values only systems.stats
// reports. Units missing from systems.stats keep these values zero.
func mergeStats(ls localStats, ss systemStats) (Stats, []decodeIssue) {
	var c collector
	for i := range ls.Encoders {
		e := &ls.Encoders[i]
		if m := ss.encoders[e.ID]; m != nil {
			ec := c.scope(string(e.ID))
			m.complete(ec, &e.EncoderMetrics, false)
			c.merge(ec)
		}
		if m := ss.decoders[e.Receiver.ID]; m != nil && e.Receiver.ID != "" {
			rc := c.scope(string(e.Receiver.ID))
			m.complete(rc, &e.Receiver, true)
			c.merge(rc)
		}
	}
	for i := range ls.Decoders {
		d := &ls.Decoders[i]
		if m := ss.decoders[d.ID]; m != nil {
			dc := c.scope(string(d.ID))
			m.complete(dc, &d.DecoderMetrics, false)
			c.merge(dc)
		}
		if m := ss.encoders[d.Sender.ID]; m != nil && d.Sender.ID != "" {
			sc := c.scope(string(d.Sender.ID))
			m.complete(sc, &d.Sender, true)
			c.merge(sc)
		}
	}
	stats := Stats{
		System:     ls.System,
		Time:       ls.Time,
		Health:     ss.health,
		Interfaces: ls.Interfaces,
		Encoders:   ls.Encoders,
		Decoders:   ls.Decoders,
		Peers:      slices.Clone(ss.peers),
	}
	return stats, c.issues
}
