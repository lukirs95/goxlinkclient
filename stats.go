package xlinkclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Stats combines the unit and interface statistics of systems.localStats with
// the system health from systems.stats. A Stats value is delivered for every
// systems.localStats message and carries the most recent health.
type Stats struct {
	System SystemID
	Time   time.Time
	// Health is the most recent system health. It is zero until the first
	// systems.stats message has been received.
	Health     Health
	Interfaces []InterfaceStats
	Encoders   []EncoderStats
	Decoders   []DecoderStats
	// Peers are the remote systems reported by systems.stats.
	Peers []PeerStats
}

// Health is the state of a system's hardware and services.
type Health struct {
	PTP          bool
	PTPSync      bool
	PTPSyncLocal bool
	NMOS         bool
	Uptime       time.Duration
	// CPU is the CPU load in percent.
	CPU int
	// CPUTemp and SystemTemp are temperatures in degrees Celsius.
	CPUTemp             int
	SystemTemp          int
	RunningVideos       int
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

// XLinkStats are the transport statistics of an XLink connection.
type XLinkStats struct {
	RTT        time.Duration
	P2P        int64
	Resent     int64
	ResentDrop int64
	Drop       int64
}

// EncoderMetrics are the statistics reported for an XLink encoder, either for
// a local encoder or as the sender of a local decoder.
type EncoderMetrics struct {
	ID       UnitID
	Running  bool
	Uptime   time.Duration
	InputFPS int
	XLink    XLinkStats
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
	ID        UnitID
	Running   bool
	Uptime    time.Duration
	OutputFPS int
	// RX and TX are in Mbps.
	RX      float64
	TX      float64
	XLink   XLinkStats
	Video   VideoStats
	Audio   AudioStats
	Buffer  BufferStats
	Receive ReceiveStats
}

// DecoderStats are the statistics of a local XLink decoder and its sender.
type DecoderStats struct {
	DecoderMetrics
	// Sender is reported by the encoder this decoder receives from. Its ID is
	// empty if no sender is selected.
	Sender EncoderMetrics
}

// VideoStats count decoded video frames.
type VideoStats struct {
	Corrected    int64
	Dropped      int64
	FECCorrected int64
	FECFailed    int64
	Missing      int64
}

// AudioStats count decoded audio frames.
type AudioStats struct {
	Dropped  int64
	Missing  int64
	Unsynced int64
}

// BufferStats describe the playout buffer.
type BufferStats struct {
	Size    time.Duration
	Min     time.Duration
	Warning bool
}

// ReceiveStats describe the received packet stream.
type ReceiveStats struct {
	Delay     time.Duration
	Expected  int64
	Received  int64
	Lost      int64
	MaxLost   int64
	Late      int64
	JitterAvg time.Duration
	JitterMin time.Duration
	JitterMax time.Duration
	// Health and TotalHealth are the share of good packets in percent, for the
	// last interval and since the start.
	Health      float64
	TotalHealth float64
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

func microsDuration(us float64) time.Duration {
	return time.Duration(us * float64(time.Microsecond))
}

func secondsDuration(s int) time.Duration {
	return time.Duration(s) * time.Second
}

// Wire types of systems.localStats.

type wireLocalStats struct {
	SysID string          `json:"sysid"`
	Time  flexTime        `json:"time"`
	Data  []wireStatsItem `json:"data"`
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

type wireXLinkStats struct {
	RTT        flexFloat `json:"rtt"`
	P2P        flexInt   `json:"p2p"`
	Resent     flexInt   `json:"resent"`
	ResentDrop flexInt   `json:"resentDrop"`
	Drop       flexInt   `json:"drop"`
}

func (x *wireXLinkStats) convert(c *collector) XLinkStats {
	xc := c.scope("xLink")
	s := XLinkStats{
		RTT:        millisDuration(xc.float("rtt", &x.RTT)),
		P2P:        int64(xc.int("p2p", &x.P2P)),
		Resent:     int64(xc.int("resent", &x.Resent)),
		ResentDrop: int64(xc.int("resentDrop", &x.ResentDrop)),
		Drop:       int64(xc.int("drop", &x.Drop)),
	}
	c.merge(xc)
	return s
}

type wireEncoderMetrics struct {
	ID      string         `json:"id"`
	Running flexBool       `json:"running"`
	UpTime  flexInt        `json:"upTime"`
	VInFps  flexInt        `json:"vInFps"`
	XLink   wireXLinkStats `json:"xLink"`
}

func (m *wireEncoderMetrics) convert(c *collector, id string) EncoderMetrics {
	if m.ID != "" {
		id = m.ID
	}
	return EncoderMetrics{
		ID:       unitID(id),
		Running:  c.bool("running", &m.Running),
		Uptime:   secondsDuration(c.int("upTime", &m.UpTime)),
		InputFPS: c.int("vInFps", &m.VInFps),
		XLink:    m.XLink.convert(c),
	}
}

type wireEncoderStats struct {
	wireEncoderMetrics
	Receiver wireDecoderMetrics `json:"receiver"`
}

type wireDecoderMetrics struct {
	ID      string         `json:"id"`
	Running flexBool       `json:"running"`
	UpTime  flexInt        `json:"upTime"`
	VOutFps flexInt        `json:"vOutFps"`
	Mbps    wireMbps       `json:"mbps"`
	XLink   wireXLinkStats `json:"xLink"`
	VDstats struct {
		Corr         flexInt `json:"corr"`
		Drop         flexInt `json:"drop"`
		FECCorrected flexInt `json:"fecCorrected"`
		FECNOK       flexInt `json:"fecnok"`
		Missing      flexInt `json:"missing"`
	} `json:"vDstats"`
	ADstats struct {
		Drop   flexInt `json:"drop"`
		Miss   flexInt `json:"miss"`
		Unsync flexInt `json:"unsync"`
	} `json:"aDstats"`
	VPbuffer struct {
		Buffer  flexFloat `json:"buffer"`
		Min     flexFloat `json:"min"`
		Warning flexBool  `json:"warning"`
	} `json:"vPbuffer"`
	VRstats struct {
		Delay    flexFloat `json:"delay"`
		Expected flexInt   `json:"expected"`
		Received flexInt   `json:"received"`
		Loss     flexInt   `json:"loss"`
		MaxLoss  flexInt   `json:"maxloss"`
		Late     flexInt   `json:"late"`
		JitAvg   flexFloat `json:"jit_avg_us"`
		JitMin   flexFloat `json:"jit_min_us"`
		JitMax   flexFloat `json:"jit_max_us"`
		Stat     flexFloat `json:"stat"`
		StatTot  flexFloat `json:"statTot"`
	} `json:"vRstats"`
}

type wireMbps struct {
	RX flexFloat `json:"rx"`
	TX flexFloat `json:"tx"`
}

func (m *wireDecoderMetrics) convert(c *collector, id string) DecoderMetrics {
	if m.ID != "" {
		id = m.ID
	}
	d := DecoderMetrics{
		ID:        unitID(id),
		Running:   c.bool("running", &m.Running),
		Uptime:    secondsDuration(c.int("upTime", &m.UpTime)),
		OutputFPS: c.int("vOutFps", &m.VOutFps),
		RX:        c.float("mbps.rx", &m.Mbps.RX),
		TX:        c.float("mbps.tx", &m.Mbps.TX),
		XLink:     m.XLink.convert(c),
	}

	v := &m.VDstats
	d.Video = VideoStats{
		Corrected:    int64(c.int("vDstats.corr", &v.Corr)),
		Dropped:      int64(c.int("vDstats.drop", &v.Drop)),
		FECCorrected: int64(c.int("vDstats.fecCorrected", &v.FECCorrected)),
		FECFailed:    int64(c.int("vDstats.fecnok", &v.FECNOK)),
		Missing:      int64(c.int("vDstats.missing", &v.Missing)),
	}
	a := &m.ADstats
	d.Audio = AudioStats{
		Dropped:  int64(c.int("aDstats.drop", &a.Drop)),
		Missing:  int64(c.int("aDstats.miss", &a.Miss)),
		Unsynced: int64(c.int("aDstats.unsync", &a.Unsync)),
	}
	b := &m.VPbuffer
	d.Buffer = BufferStats{
		Size:    millisDuration(c.float("vPbuffer.buffer", &b.Buffer)),
		Min:     millisDuration(c.float("vPbuffer.min", &b.Min)),
		Warning: c.bool("vPbuffer.warning", &b.Warning),
	}
	r := &m.VRstats
	d.Receive = ReceiveStats{
		Delay:       millisDuration(c.float("vRstats.delay", &r.Delay)),
		Expected:    int64(c.int("vRstats.expected", &r.Expected)),
		Received:    int64(c.int("vRstats.received", &r.Received)),
		Lost:        int64(c.int("vRstats.loss", &r.Loss)),
		MaxLost:     int64(c.int("vRstats.maxloss", &r.MaxLoss)),
		Late:        int64(c.int("vRstats.late", &r.Late)),
		JitterAvg:   microsDuration(c.float("vRstats.jit_avg_us", &r.JitAvg)),
		JitterMin:   microsDuration(c.float("vRstats.jit_min_us", &r.JitMin)),
		JitterMax:   microsDuration(c.float("vRstats.jit_max_us", &r.JitMax)),
		Health:      c.float("vRstats.stat", &r.Stat),
		TotalHealth: c.float("vRstats.statTot", &r.StatTot),
	}
	return d
}

type wireDecoderStats struct {
	wireDecoderMetrics
	Sender wireEncoderMetrics `json:"sender"`
}

// localStats is a decoded systems.localStats message.
type localStats struct {
	System     SystemID
	Time       time.Time
	Interfaces []InterfaceStats
	Encoders   []EncoderStats
	Decoders   []DecoderStats
}

// decodeLocalStats decodes a systems.localStats message. Items of unknown
// types, e.g. SRT or NDI units, are ignored.
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
				EncoderMetrics: s.wireEncoderMetrics.convert(ic, item.ID),
				Receiver:       s.Receiver.convert(rc, ""),
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
				DecoderMetrics: s.wireDecoderMetrics.convert(ic, item.ID),
				Sender:         s.Sender.convert(sc, ""),
			})
			ic.merge(sc)
		}
		c.merge(ic)
	}
	return ls, c.issues, nil
}

// Wire types of systems.stats. Only the health of the local and the remote
// systems is used; unit statistics come from systems.localStats.

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
	PTPSync      flexBool  `json:"ptpSync"`
	PTPSyncLocal flexBool  `json:"ptpSyncLocal"`
	PTP          flexBool  `json:"ptp"`
	NMOS         flexBool  `json:"nmos"`
	OSUpTime     flexInt   `json:"osUpTime"`
	CPU          flexInt   `json:"cpu"`
	CPUTemp      flexInt   `json:"cpuTemp"`
	SysTemp      flexInt   `json:"sysTemp"`
	VTotRun      flexInt   `json:"vTotRun"`
	VLic         flexInt   `json:"vLic"`
	VLicDec      flexInt   `json:"vLicDec"`
	VLicUsed     flexInt   `json:"vLicUsed"`
	VLicUsedDec  flexInt   `json:"vLicUsedDec"`
	RTT          flexFloat `json:"rtt"`
}

func (h *wireHealth) convert(c *collector) Health {
	return Health{
		PTP:                 c.bool("ptp", &h.PTP),
		PTPSync:             c.bool("ptpSync", &h.PTPSync),
		PTPSyncLocal:        c.bool("ptpSyncLocal", &h.PTPSyncLocal),
		NMOS:                c.bool("nmos", &h.NMOS),
		Uptime:              secondsDuration(c.int("osUpTime", &h.OSUpTime)),
		CPU:                 c.int("cpu", &h.CPU),
		CPUTemp:             c.int("cpuTemp", &h.CPUTemp),
		SystemTemp:          c.int("sysTemp", &h.SysTemp),
		RunningVideos:       c.int("vTotRun", &h.VTotRun),
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

// decodeSystemStats decodes the health of the local and remote systems from a
// systems.stats message.
func decodeSystemStats(params json.RawMessage) (Health, []PeerStats, []decodeIssue, error) {
	var w wireSystemStats
	if err := json.Unmarshal(params, &w); err != nil {
		return Health{}, nil, nil, fmt.Errorf("%w: %w", errBadStats, err)
	}

	var c collector
	health, _, _ := findHealth(&c, w.SysID, w.Data.Local)
	peers := make([]PeerStats, 0, len(w.Data.Remote))
	for _, r := range w.Data.Remote {
		h, rtt, ok := findHealth(&c, r.SysID, r.Data)
		if !ok {
			continue
		}
		peers = append(peers, PeerStats{ID: SystemID(r.SysID), Health: h, RTT: millisDuration(rtt)})
	}
	return health, peers, c.issues, nil
}
