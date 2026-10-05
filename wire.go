package xlinkclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// The wire types mirror the JSON sent with systems.full and systems.update.
// They hold the persistent state that deltas are merged into and are
// converted to the public model on every snapshot. Only fields used by the
// public model are declared; everything else is ignored by encoding/json.

type wireParams struct {
	SysID  string   `json:"sysid"`
	DataID flexInt  `json:"dataid"`
	Data   wireData `json:"data"`
}

type wireData struct {
	Local  wireLocal                      `json:"local"`
	Remote keyedList[wirePeer, *wirePeer] `json:"remote"`
}

type wireLocal struct {
	SysID       string                               `json:"sysid"`
	Name        string                               `json:"name"`
	SysVer      string                               `json:"sysVer"`
	SysBeta     flexBool                             `json:"sysBeta"`
	ST2110      flexBool                             `json:"st2110"`
	NeedReboot  flexBool                             `json:"needReboot"`
	SysST       flexTime                             `json:"sysST"`
	Profile     string                               `json:"profile"`
	ProfileName string                               `json:"profileName"`
	Ports       wirePorts                            `json:"ports"`
	MTUTrunk    flexInt                              `json:"mtuTrunk"`
	Enc         keyedList[wireEncoder, *wireEncoder] `json:"enc"`
	Dec         keyedList[wireDecoder, *wireDecoder] `json:"dec"`
	L2S         keyedList[wireTrunk, *wireTrunk]     `json:"l2s"`
	Network     wireNetwork                          `json:"network"`
}

type wirePorts struct {
	SysPort   flexInt  `json:"sysPort"`
	SysOn     flexBool `json:"sysOn"`
	PortsOn   flexBool `json:"portsOn"`
	PortsFrom flexInt  `json:"portsFrom"`
	PortsTo   flexInt  `json:"portsTo"`
}

type wireNetwork struct {
	Nets keyedList[wireInterface, *wireInterface] `json:"nets"`
}

// wireStreamValues are the primary SMPTE ST 2110 keys shared by encoders,
// decoders and their counterparts.
type wireStreamValues struct {
	V2110NetPri        string   `json:"v2110NetPri"`
	V2110NetPriEnabled flexBool `json:"v2110NetPriEnabled"`
	V2110NetPriIP      string   `json:"v2110NetPriIp"`
	V2110NetPriPort    flexInt  `json:"v2110NetPriPort"`
	A2110NetPri        string   `json:"a2110NetPri"`
	A2110NetPriEnabled flexBool `json:"a2110NetPriEnabled"`
	A2110NetPriIP      string   `json:"a2110NetPriIp"`
	A2110NetPriPort    flexInt  `json:"a2110NetPriPort"`
}

func (v *wireStreamValues) streams(c *collector) (video, audio Stream) {
	video = Stream{
		Interface: iface(v.V2110NetPri),
		Enabled:   c.bool("v2110NetPriEnabled", &v.V2110NetPriEnabled),
		Address:   v.V2110NetPriIP,
		Port:      c.int("v2110NetPriPort", &v.V2110NetPriPort),
	}
	audio = Stream{
		Interface: iface(v.A2110NetPri),
		Enabled:   c.bool("a2110NetPriEnabled", &v.A2110NetPriEnabled),
		Address:   v.A2110NetPriIP,
		Port:      c.int("a2110NetPriPort", &v.A2110NetPriPort),
	}
	return video, audio
}

type wireEncoder struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Enabled  flexBool          `json:"enabled"`
	Values   wireEncoderValues `json:"values"`
	Receiver wireLinked        `json:"receiver"`
}

func (e *wireEncoder) key() string { return e.ID }

type wireEncoderValues struct {
	wireStreamValues
	VIn      string   `json:"vIn"`
	AIn      string   `json:"aIn"`
	VCard    flexInt  `json:"vCard"`
	VCodec   flexInt  `json:"vCodec"`
	VTBR     flexInt  `json:"vTBR"`
	VFEC     flexInt  `json:"vFEC"`
	XLink    flexBool `json:"xLink"`
	XLinkP2P flexBool `json:"xLinkp2p"`
	Running  flexBool `json:"running"`
	StartT   flexTime `json:"startT"`
}

func (e *wireEncoder) toEncoder(c *collector) Encoder {
	v := &e.Values
	vc := c.scope("values")
	video, audio := v.streams(vc)
	enc := Encoder{
		ID:        UnitID(e.ID),
		Name:      e.Name,
		Enabled:   c.bool("enabled", &e.Enabled),
		Running:   vc.bool("running", &v.Running),
		StartedAt: vc.time("startT", &v.StartT),
		Card:      VideoCard(vc.int("vCard", &v.VCard)),
		VideoIn:   Signal(v.VIn),
		AudioIn:   Signal(v.AIn),
		Codec:     VideoCodec(vc.int("vCodec", &v.VCodec)),
		Bitrate:   vc.int("vTBR", &v.VTBR),
		FECLevel:  vc.int("vFEC", &v.VFEC),
		Video2110: video,
		Audio2110: audio,
		XLink:     vc.bool("xLink", &v.XLink),
		P2P:       vc.bool("xLinkp2p", &v.XLinkP2P),
	}
	c.merge(vc)
	rc := c.scope("receiver")
	enc.Receiver = e.Receiver.toLinked(rc, true)
	c.merge(rc)
	return enc
}

type wireDecoder struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Enabled flexBool          `json:"enabled"`
	Values  wireDecoderValues `json:"values"`
	Sender  wireLinked        `json:"sender"`
}

func (d *wireDecoder) key() string { return d.ID }

type wireDecoderValues struct {
	wireStreamValues
	VIn       string   `json:"vIn"`
	AIn       string   `json:"aIn"`
	VOut      string   `json:"vOut"`
	AOut      string   `json:"aOut"`
	AInCh     flexInt  `json:"aInCh"`
	VCard     flexInt  `json:"vCard"`
	VBnoInOn  flexBool `json:"vBnoInOn"`
	VBnoIn    flexInt  `json:"vBnoIn"`
	VBufferOn flexBool `json:"vBufferOn"`
	VBuffer   flexInt  `json:"vBuffer"`
	VFRCOn    flexBool `json:"vFRCOn"`
	XLink     flexBool `json:"xLink"`
	XLinkP2P  flexBool `json:"xLinkp2p"`
	Running   flexBool `json:"running"`
	StartT    flexTime `json:"startT"`
}

func (d *wireDecoder) toDecoder(c *collector) Decoder {
	v := &d.Values
	vc := c.scope("values")
	video, audio := v.streams(vc)
	dec := Decoder{
		ID:             UnitID(d.ID),
		Name:           d.Name,
		Enabled:        c.bool("enabled", &d.Enabled),
		Running:        vc.bool("running", &v.Running),
		StartedAt:      vc.time("startT", &v.StartT),
		Card:           VideoCard(vc.int("vCard", &v.VCard)),
		VideoIn:        Signal(v.VIn),
		AudioIn:        Signal(v.AIn),
		VideoOut:       Signal(v.VOut),
		AudioOut:       Signal(v.AOut),
		AudioChannels:  vc.int("aInCh", &v.AInCh),
		SignalGenOn:    vc.bool("vBnoInOn", &v.VBnoInOn),
		SignalGenDelay: time.Duration(vc.int("vBnoIn", &v.VBnoIn)) * time.Second,
		BufferOn:       vc.bool("vBufferOn", &v.VBufferOn),
		Buffer:         vc.int("vBuffer", &v.VBuffer),
		FPSSync:        vc.bool("vFRCOn", &v.VFRCOn),
		Video2110:      video,
		Audio2110:      audio,
		XLink:          vc.bool("xLink", &v.XLink),
		P2P:            vc.bool("xLinkp2p", &v.XLinkP2P),
	}
	c.merge(vc)
	sc := c.scope("sender")
	dec.Sender = d.Sender.toLinked(sc, false)
	c.merge(sc)
	return dec
}

// wireLinked is the counterpart embedded in a local encoder ("receiver") or
// decoder ("sender").
type wireLinked struct {
	ID      string           `json:"id"`
	SysName string           `json:"sysName"`
	Name    string           `json:"name"`
	Values  wireLinkedValues `json:"values"`
}

type wireLinkedValues struct {
	VIn        string   `json:"vIn"`
	AIn        string   `json:"aIn"`
	VOut       string   `json:"vOut"`
	AOut       string   `json:"aOut"`
	Connected  flexBool `json:"connected"`
	XLinkP2P   flexBool `json:"xLinkp2p"`
	OnLocalNet flexBool `json:"onLocalNet"`
	Running    flexBool `json:"running"`
}

// toLinked converts the counterpart. For a receiver its outputs are the
// relevant signals, for a sender its inputs.
func (l *wireLinked) toLinked(c *collector, receiver bool) LinkedUnit {
	v := &l.Values
	vc := c.scope("values")
	u := LinkedUnit{
		ID:         unitID(l.ID),
		Name:       l.Name,
		SystemName: l.SysName,
		Running:    vc.bool("running", &v.Running),
		Connected:  vc.bool("connected", &v.Connected),
		P2P:        vc.bool("xLinkp2p", &v.XLinkP2P),
		OnLocalNet: vc.bool("onLocalNet", &v.OnLocalNet),
		Video:      Signal(v.VIn),
		Audio:      Signal(v.AIn),
	}
	if receiver {
		u.Video, u.Audio = Signal(v.VOut), Signal(v.AOut)
	}
	c.merge(vc)
	return u
}

type wireInterface struct {
	ID           string   `json:"id"`
	MAC          string   `json:"mac"`
	DHCP         flexBool `json:"dhcp"`
	IP           string   `json:"ip"`
	Mask         string   `json:"mask"`
	Gate         string   `json:"gate"`
	DNS1         string   `json:"dns1"`
	DNS2         string   `json:"dns2"`
	Enabled      flexBool `json:"enabled"`
	Link         flexBool `json:"link"`
	LinkTime     flexTime `json:"linkTime"`
	Speed        string   `json:"speed"`
	Active       flexBool `json:"active"`
	Internet     flexBool `json:"internet"`
	GatePing     flexBool `json:"gatePing"`
	AdminOnly    flexBool `json:"adminOnly"`
	Admin        flexBool `json:"admin"`
	AdminSSLOnly flexBool `json:"adminSslOnly"`
	IGMP         flexBool `json:"igmp"`
	NMOS         flexBool `json:"nmos"`
	Default      flexBool `json:"default"`
	DefaultLan   flexBool `json:"defaultLan"`
	Backup       flexBool `json:"backup"`
	L2SID        string   `json:"l2sId"`
}

func (i *wireInterface) key() string { return i.ID }

func (i *wireInterface) toInterface(c *collector) Interface {
	trunk := TrunkID(i.L2SID)
	if i.L2SID == "none" {
		trunk = ""
	}
	return Interface{
		ID:              i.ID,
		MAC:             i.MAC,
		DHCP:            c.bool("dhcp", &i.DHCP),
		IP:              i.IP,
		Mask:            i.Mask,
		Gateway:         i.Gate,
		DNS:             [2]string{i.DNS1, i.DNS2},
		Enabled:         c.bool("enabled", &i.Enabled),
		LinkUp:          c.bool("link", &i.Link),
		LinkSince:       c.time("linkTime", &i.LinkTime),
		Speed:           i.Speed,
		Active:          c.bool("active", &i.Active),
		Internet:        c.bool("internet", &i.Internet),
		GatewayPing:     c.bool("gatePing", &i.GatePing),
		AdminOnly:       c.bool("adminOnly", &i.AdminOnly),
		WebAdmin:        c.bool("admin", &i.Admin),
		HTTPSOnly:       c.bool("adminSslOnly", &i.AdminSSLOnly),
		IGMP:            c.bool("igmp", &i.IGMP),
		NMOS:            c.bool("nmos", &i.NMOS),
		DefaultExternal: c.bool("default", &i.Default),
		DefaultLAN:      c.bool("defaultLan", &i.DefaultLan),
		Backup:          c.bool("backup", &i.Backup),
		Trunk:           trunk,
	}
}

type wireTrunk struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Running flexBool        `json:"running"`
	Values  wireTrunkValues `json:"values"`
}

func (t *wireTrunk) key() string { return t.ID }

type wireTrunkValues struct {
	Eth        string   `json:"eth"`
	Master     flexBool `json:"master"`
	AutoStart  flexBool `json:"autoStart"`
	Multicast  flexBool `json:"multicast"`
	RMcast     flexBool `json:"rmcast"`
	Encryption flexBool `json:"encryption"`
	L2MTUOn    flexBool `json:"l2mtuOn"`
	L2MTU      flexInt  `json:"l2mtu"`
}

func (t *wireTrunk) toTrunk(c *collector) Trunk {
	v := &t.Values
	vc := c.scope("values")
	trunk := Trunk{
		ID:                TrunkID(t.ID),
		Name:              t.Name,
		Running:           c.bool("running", &t.Running),
		Interface:         iface(v.Eth),
		Master:            vc.bool("master", &v.Master),
		AutoStart:         vc.bool("autoStart", &v.AutoStart),
		Multicast:         vc.bool("multicast", &v.Multicast),
		ReliableMulticast: vc.bool("rmcast", &v.RMcast),
		Encryption:        vc.bool("encryption", &v.Encryption),
	}
	mtu := vc.int("l2mtu", &v.L2MTU)
	if vc.bool("l2mtuOn", &v.L2MTUOn) {
		trunk.MTU = mtu
	}
	c.merge(vc)
	return trunk
}

type wirePeer struct {
	SysID      string                                 `json:"sysid"`
	Name       string                                 `json:"name"`
	SysVer     string                                 `json:"sysVer"`
	ST2110     flexBool                               `json:"st2110"`
	Connected  flexBool                               `json:"connected"`
	XLinkP2P   flexBool                               `json:"xLinkp2p"`
	OnLocalNet flexBool                               `json:"onLocalNet"`
	ConIP      string                                 `json:"conIp"`
	ConPort    flexInt                                `json:"conPort"`
	ConnectedT flexTime                               `json:"connectedT"`
	LastSeenT  flexTime                               `json:"lastSeenT"`
	ManIP      string                                 `json:"manIp"`
	ManIPSec   string                                 `json:"manIpSec"`
	ManPort    flexInt                                `json:"manPort"`
	ManAutCon  flexBool                               `json:"manAutCon"`
	Enc        keyedList[wirePeerUnit, *wirePeerUnit] `json:"enc"`
	Dec        keyedList[wirePeerUnit, *wirePeerUnit] `json:"dec"`
	Network    wireNetwork                            `json:"network"`
}

func (p *wirePeer) key() string { return p.SysID }

// wirePeerUnit is the summary of a unit on a remote system.
type wirePeerUnit struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Enabled  flexBool `json:"enabled"`
	Running  flexBool `json:"running"`
	Receiver string   `json:"receiver"`
	Sender   string   `json:"sender"`
}

func (u *wirePeerUnit) key() string { return u.ID }

func (u *wirePeerUnit) toPeerUnit(c *collector) PeerUnit {
	linked := u.Receiver
	if linked == "" {
		linked = u.Sender
	}
	return PeerUnit{
		ID:      UnitID(u.ID),
		Name:    u.Name,
		Enabled: c.bool("enabled", &u.Enabled),
		Running: c.bool("running", &u.Running),
		Linked:  unitID(linked),
	}
}

func (p *wirePeer) toPeer(c *collector) Peer {
	peer := Peer{
		ID:             SystemID(p.SysID),
		Name:           p.Name,
		Version:        p.SysVer,
		ST2110:         c.bool("st2110", &p.ST2110),
		Connected:      c.bool("connected", &p.Connected),
		P2P:            c.bool("xLinkp2p", &p.XLinkP2P),
		OnLocalNet:     c.bool("onLocalNet", &p.OnLocalNet),
		IP:             p.ConIP,
		Port:           c.int("conPort", &p.ConPort),
		ConnectedSince: c.time("connectedT", &p.ConnectedT),
		LastSeen:       c.time("lastSeenT", &p.LastSeenT),
		ManualIP: ManualIP{
			Primary:     p.ManIP,
			Secondary:   p.ManIPSec,
			Port:        c.int("manPort", &p.ManPort),
			AutoConnect: c.bool("manAutCon", &p.ManAutCon),
		},
	}
	peer.Encoders = convertList(c, "enc", &p.Enc, (*wirePeerUnit).toPeerUnit)
	peer.Decoders = convertList(c, "dec", &p.Dec, (*wirePeerUnit).toPeerUnit)
	peer.Interfaces = convertList(c, "nets", &p.Network.Nets, (*wireInterface).toInterface)
	return peer
}

// convertList converts every element of a keyed list, scoping issues by list
// name and element key.
func convertList[T any, P keyedElement[T], R any](c *collector, name string, l *keyedList[T, P], convert func(P, *collector) R) []R {
	lc := c.scope(name)
	lc.add(l.takeIssues()...)
	out := make([]R, 0, len(l.items))
	for _, item := range l.items {
		ec := lc.scope(P(item).key())
		out = append(out, convert(item, ec))
		lc.merge(ec)
	}
	c.merge(lc)
	return out
}

func (p *wireParams) toSystem(c *collector) System {
	l := &p.Data.Local
	sys := System{
		ID:          SystemID(l.SysID),
		Name:        l.Name,
		Version:     l.SysVer,
		Beta:        c.bool("sysBeta", &l.SysBeta),
		ST2110:      c.bool("st2110", &l.ST2110),
		NeedsReboot: c.bool("needReboot", &l.NeedReboot),
		StartedAt:   c.time("sysST", &l.SysST),
		Profile:     Profile{ID: l.Profile, Name: l.ProfileName},
		Ports: Ports{
			SystemStatic: c.bool("ports.sysOn", &l.Ports.SysOn),
			SystemPort:   c.int("ports.sysPort", &l.Ports.SysPort),
			DataStatic:   c.bool("ports.portsOn", &l.Ports.PortsOn),
			DataFrom:     c.int("ports.portsFrom", &l.Ports.PortsFrom),
			DataTo:       c.int("ports.portsTo", &l.Ports.PortsTo),
		},
		TrunkMTU: c.int("mtuTrunk", &l.MTUTrunk),
	}
	if sys.ID == "" {
		sys.ID = SystemID(p.SysID)
	}
	sys.Encoders = convertList(c, "enc", &l.Enc, (*wireEncoder).toEncoder)
	sys.Decoders = convertList(c, "dec", &l.Dec, (*wireDecoder).toDecoder)
	sys.Trunks = convertList(c, "l2s", &l.L2S, (*wireTrunk).toTrunk)
	sys.Interfaces = convertList(c, "nets", &l.Network.Nets, (*wireInterface).toInterface)
	sys.Peers = convertList(c, "remote", &p.Data.Remote, (*wirePeer).toPeer)
	return sys
}

// errNoState is returned when an update arrives before the full state.
var errNoState = errors.New("xlinkclient: update received before full state")

// state is the merged wire state of one system.
type state struct {
	wire   wireParams
	loaded bool
}

// applyFull replaces the state with a systems.full message.
func (s *state) applyFull(params json.RawMessage) error {
	var w wireParams
	if err := decodeParams(params, &w); err != nil {
		return err
	}
	s.wire = w
	s.loaded = true
	return nil
}

// applyUpdate merges a systems.update message into the state. gap reports
// whether messages were lost since the previous one, judged by dataid.
func (s *state) applyUpdate(params json.RawMessage) (gap bool, err error) {
	if !s.loaded {
		return false, errNoState
	}
	prev := s.wire.DataID.Value
	if err := decodeParams(params, &s.wire); err != nil {
		return false, err
	}
	return s.wire.DataID.Value != prev+1, nil
}

// snapshot converts the state to the public model. It returns the issues
// found since the previous snapshot.
func (s *state) snapshot() (System, []decodeIssue) {
	var c collector
	sys := s.wire.toSystem(&c)
	return sys, c.issues
}

// decodeParams decodes params into w. Type mismatches of single fields are
// tolerated by encoding/json, which keeps decoding the rest; they are not
// treated as errors here. Only malformed JSON fails.
func decodeParams(params json.RawMessage, w *wireParams) error {
	err := json.Unmarshal(params, w)
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("xlinkclient: decode system message: %w", err)
	}
	return nil
}
