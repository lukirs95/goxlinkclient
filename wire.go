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
	SysID       string                                   `json:"sysid"`
	Name        string                                   `json:"name"`
	SysVer      string                                   `json:"sysVer"`
	ST2110      flexBool                                 `json:"st2110"`
	NeedReboot  flexBool                                 `json:"needReboot"`
	CloudXLink  flexBool                                 `json:"cloudXlink"`
	Profile     string                                   `json:"profile"`
	ProfileName string                                   `json:"profileName"`
	Ports       wirePorts                                `json:"ports"`
	MTUTrunk    flexInt                                  `json:"mtuTrunk"`
	Enc         keyedList[wireEncoder, *wireEncoder]     `json:"enc"`
	Dec         keyedList[wireDecoder, *wireDecoder]     `json:"dec"`
	SRT         keyedList[wireMediaUnit, *wireMediaUnit] `json:"srt"`
	NDI         keyedList[wireMediaUnit, *wireMediaUnit] `json:"ndi"`
	L2S         keyedList[wireTrunk, *wireTrunk]         `json:"l2s"`
	Network     wireNetwork                              `json:"network"`
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
	VIn       string   `json:"vIn"`
	AIn       string   `json:"aIn"`
	VCard     flexInt  `json:"vCard"`
	VCardConn string   `json:"vCardConn"`
	VHDR      flexInt  `json:"vHDR"`
	VCodec    flexInt  `json:"vCodec"`
	VCRFOn    flexBool `json:"vCRFOn"`
	VTBR      flexInt  `json:"vTBR"`
	VFEC      flexInt  `json:"vFEC"`
	wireSourceValues
	XLink    flexBool `json:"xLink"`
	XLinkP2P flexBool `json:"xLinkp2p"`
	Running  flexBool `json:"running"`
}

func (e *wireEncoder) toEncoder(c *collector) Encoder {
	v := &e.Values
	vc := c.scope("values")
	video, audio := v.streams(vc)
	enc := Encoder{
		ID:                 UnitID(e.ID),
		Name:               e.Name,
		Enabled:            c.bool("enabled", &e.Enabled),
		Running:            vc.bool("running", &v.Running),
		Card:               VideoCard(vc.int("vCard", &v.VCard)),
		Connector:          v.VCardConn,
		VideoIn:            Signal(v.VIn),
		AudioIn:            Signal(v.AIn),
		HDR:                HDRMode(vc.int("vHDR", &v.VHDR)),
		NDISource:          v.NDISource,
		NDISourceName:      v.NDISourceName,
		NDISourceConnected: vc.bool("ndiSourceCon", &v.NDISourceCon),
		MultiPointSource:   v.multiPointSource(),
		Codec:              VideoCodec(vc.int("vCodec", &v.VCodec)),
		CRF:                vc.bool("vCRFOn", &v.VCRFOn),
		Bitrate:            vc.int("vTBR", &v.VTBR),
		FECLevel:           vc.int("vFEC", &v.VFEC),
		Video2110:          video,
		Audio2110:          audio,
		XLink:              vc.bool("xLink", &v.XLink),
		P2P:                vc.bool("xLinkp2p", &v.XLinkP2P),
	}
	c.merge(vc)
	rc := c.scope("receiver")
	enc.Receiver = e.Receiver.toReceiver(rc)
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
	VCard     flexInt  `json:"vCard"`
	VCardConn string   `json:"vCardConn"`
	VHDR      flexInt  `json:"vHDR"`
	VRef      flexBool `json:"vRef"`
	AVSync    flexBool `json:"avSync"`
	AVUnSync  flexInt  `json:"avUnSync"`
	VBnoInOn  flexBool `json:"vBnoInOn"`
	VBnoIn    flexInt  `json:"vBnoIn"`
	VFRCOn    flexBool `json:"vFRCOn"`
	XLink     flexBool `json:"xLink"`
	XLinkP2P  flexBool `json:"xLinkp2p"`
	Running   flexBool `json:"running"`
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
		Card:           VideoCard(vc.int("vCard", &v.VCard)),
		Connector:      v.VCardConn,
		VideoIn:        Signal(v.VIn),
		AudioIn:        Signal(v.AIn),
		VideoOut:       Signal(v.VOut),
		AudioOut:       Signal(v.AOut),
		HDR:            HDRMode(vc.int("vHDR", &v.VHDR)),
		RefLocked:      vc.bool("vRef", &v.VRef),
		AVSync:         vc.bool("avSync", &v.AVSync),
		AVDelay:        time.Duration(vc.int("avUnSync", &v.AVUnSync)) * time.Millisecond,
		SignalGenOn:    vc.bool("vBnoInOn", &v.VBnoInOn),
		SignalGenDelay: time.Duration(vc.int("vBnoIn", &v.VBnoIn)) * time.Second,
		FPSSync:        vc.bool("vFRCOn", &v.VFRCOn),
		Video2110:      video,
		Audio2110:      audio,
		XLink:          vc.bool("xLink", &v.XLink),
		P2P:            vc.bool("xLinkp2p", &v.XLinkP2P),
	}
	c.merge(vc)
	sc := c.scope("sender")
	dec.Sender = d.Sender.toSender(sc)
	c.merge(sc)
	return dec
}

// wireMediaUnit is an element of the "srt" and "ndi" arrays. Each array holds
// encoders and decoders alike; they are told apart by the prefix of their ID,
// which unlike "type" is present in every delta.
type wireMediaUnit struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Enabled  flexBool            `json:"enabled"`
	RunError string              `json:"runError"`
	Values   wireMediaUnitValues `json:"values"`
}

func (u *wireMediaUnit) key() string { return u.ID }

// wireMediaUnitValues holds the keys of SRT and NDI units. In systems.full the
// SRT keys are prefixed with "srt" (and the typo in srtEncrytion is the
// device's), while config and state.subscribe use the bare names.
type wireMediaUnitValues struct {
	VIn           string   `json:"vIn"`
	VOut          string   `json:"vOut"`
	VCard         flexInt  `json:"vCard"`
	VCardConn     string   `json:"vCardConn"`
	VRef          flexBool `json:"vRef"`
	AVSync        flexBool `json:"avSync"`
	AVUnSync      flexInt  `json:"avUnSync"`
	VTBR          flexInt  `json:"vTBR"`
	SRTMode       flexInt  `json:"srtMode"`
	SRTAddress    string   `json:"srtAddress"`
	SRTPort       flexInt  `json:"srtPort"`
	SRTLocalPort  flexInt  `json:"srtLocalPort"`
	SRTEncryption flexBool `json:"srtEncrytion"`
	NDISource     string   `json:"ndiSource"`
	NDISourceName string   `json:"ndiSourceName"`
	NDISourceCon  flexBool `json:"ndiSourceCon"`
	VFRCOn        flexBool `json:"vFRCOn"`
	Running       flexBool `json:"running"`
}

func (u *wireMediaUnit) srtConnection(c *collector) SRTConnection {
	v := &u.Values
	return SRTConnection{
		Mode:       SRTMode(c.int("srtMode", &v.SRTMode)),
		Address:    v.SRTAddress,
		Port:       c.int("srtPort", &v.SRTPort),
		LocalPort:  c.int("srtLocalPort", &v.SRTLocalPort),
		Encryption: c.bool("srtEncrytion", &v.SRTEncryption),
	}
}

func (u *wireMediaUnit) toSRTEncoder(c *collector) SRTEncoder {
	v := &u.Values
	vc := c.scope("values")
	enc := SRTEncoder{
		ID:        UnitID(u.ID),
		Name:      u.Name,
		Enabled:   c.bool("enabled", &u.Enabled),
		Running:   vc.bool("running", &v.Running),
		Error:     u.RunError,
		Card:      VideoCard(vc.int("vCard", &v.VCard)),
		Connector: v.VCardConn,
		VideoIn:   Signal(v.VIn),
		Bitrate:   vc.int("vTBR", &v.VTBR),
		SRT:       u.srtConnection(vc),
	}
	c.merge(vc)
	return enc
}

func (u *wireMediaUnit) toSRTDecoder(c *collector) SRTDecoder {
	v := &u.Values
	vc := c.scope("values")
	dec := SRTDecoder{
		ID:        UnitID(u.ID),
		Name:      u.Name,
		Enabled:   c.bool("enabled", &u.Enabled),
		Running:   vc.bool("running", &v.Running),
		Error:     u.RunError,
		Card:      VideoCard(vc.int("vCard", &v.VCard)),
		Connector: v.VCardConn,
		VideoIn:   Signal(v.VIn),
		VideoOut:  Signal(v.VOut),
		SRT:       u.srtConnection(vc),
	}
	c.merge(vc)
	return dec
}

func (u *wireMediaUnit) toNDIEncoder(c *collector) NDIEncoder {
	v := &u.Values
	vc := c.scope("values")
	enc := NDIEncoder{
		ID:        UnitID(u.ID),
		Name:      u.Name,
		Enabled:   c.bool("enabled", &u.Enabled),
		Running:   vc.bool("running", &v.Running),
		Card:      VideoCard(vc.int("vCard", &v.VCard)),
		Connector: v.VCardConn,
		VideoIn:   Signal(v.VIn),
		VideoOut:  Signal(v.VOut),
	}
	c.merge(vc)
	return enc
}

func (u *wireMediaUnit) toNDIDecoder(c *collector) NDIDecoder {
	v := &u.Values
	vc := c.scope("values")
	dec := NDIDecoder{
		ID:              UnitID(u.ID),
		Name:            u.Name,
		Enabled:         c.bool("enabled", &u.Enabled),
		Running:         vc.bool("running", &v.Running),
		Card:            VideoCard(vc.int("vCard", &v.VCard)),
		Connector:       v.VCardConn,
		VideoIn:         Signal(v.VIn),
		VideoOut:        Signal(v.VOut),
		Source:          v.NDISource,
		SourceName:      v.NDISourceName,
		SourceConnected: vc.bool("ndiSourceCon", &v.NDISourceCon),
		RefLocked:       vc.bool("vRef", &v.VRef),
		AVSync:          vc.bool("avSync", &v.AVSync),
		AVDelay:         time.Duration(vc.int("avUnSync", &v.AVUnSync)) * time.Millisecond,
		FPSSync:         vc.bool("vFRCOn", &v.VFRCOn),
	}
	c.merge(vc)
	return dec
}

// ofType matches media units by the type encoded in their ID.
func ofType(t UnitType) func(*wireMediaUnit) bool {
	return func(u *wireMediaUnit) bool { return UnitID(u.ID).Type() == t }
}

// wireSourceValues are the NDI and MultiPoint inputs of an XLink encoder.
type wireSourceValues struct {
	NDISource     string   `json:"ndiSource"`
	NDISourceName string   `json:"ndiSourceName"`
	NDISourceCon  flexBool `json:"ndiSourceCon"`
	MPointSender  string   `json:"mPointSender"`
}

// multiPointSource maps the device's "none" to "".
func (v *wireSourceValues) multiPointSource() string {
	if v.MPointSender == "none" {
		return ""
	}
	return v.MPointSender
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
	VIn       string   `json:"vIn"`
	AIn       string   `json:"aIn"`
	VOut      string   `json:"vOut"`
	AOut      string   `json:"aOut"`
	VCard     flexInt  `json:"vCard"`
	VCardConn string   `json:"vCardConn"`
	VHDR      flexInt  `json:"vHDR"`
	VRef      flexBool `json:"vRef"`
	AVSync    flexBool `json:"avSync"`
	AVUnSync  flexInt  `json:"avUnSync"`
	VCodec    flexInt  `json:"vCodec"`
	VCRFOn    flexBool `json:"vCRFOn"`
	VTBR      flexInt  `json:"vTBR"`
	wireSourceValues
	Connected flexBool `json:"connected"`
	Running   flexBool `json:"running"`
}

// toLinked converts the values shared by receivers and senders.
func (l *wireLinked) toLinked(c *collector) LinkedUnit {
	v := &l.Values
	return LinkedUnit{
		ID:         unitID(l.ID),
		Name:       l.Name,
		SystemName: l.SysName,
		Running:    c.bool("running", &v.Running),
		Connected:  c.bool("connected", &v.Connected),
		Card:       VideoCard(c.int("vCard", &v.VCard)),
		Connector:  v.VCardConn,
		VideoIn:    Signal(v.VIn),
		AudioIn:    Signal(v.AIn),
		HDR:        HDRMode(c.int("vHDR", &v.VHDR)),
	}
}

func (l *wireLinked) toReceiver(c *collector) LinkedReceiver {
	v := &l.Values
	vc := c.scope("values")
	r := LinkedReceiver{
		LinkedUnit: l.toLinked(vc),
		VideoOut:   Signal(v.VOut),
		AudioOut:   Signal(v.AOut),
		RefLocked:  vc.bool("vRef", &v.VRef),
		AVSync:     vc.bool("avSync", &v.AVSync),
		AVDelay:    time.Duration(vc.int("avUnSync", &v.AVUnSync)) * time.Millisecond,
	}
	c.merge(vc)
	return r
}

func (l *wireLinked) toSender(c *collector) LinkedSender {
	v := &l.Values
	vc := c.scope("values")
	s := LinkedSender{
		LinkedUnit:         l.toLinked(vc),
		NDISource:          v.NDISource,
		NDISourceConnected: vc.bool("ndiSourceCon", &v.NDISourceCon),
		MultiPointSource:   v.multiPointSource(),
		Codec:              VideoCodec(vc.int("vCodec", &v.VCodec)),
		CRF:                vc.bool("vCRFOn", &v.VCRFOn),
		Bitrate:            vc.int("vTBR", &v.VTBR),
	}
	c.merge(vc)
	return s
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
	Reboot     flexBool                               `json:"reboot"`
	RebootT    flexTime                               `json:"rebootT"`
	Shutdown   flexBool                               `json:"shutdown"`
	ShutdownT  flexTime                               `json:"shutdownT"`
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
	// The web UI shows the times only while the flags are set.
	if c.bool("reboot", &p.Reboot) {
		peer.RebootReceived = c.time("rebootT", &p.RebootT)
	}
	if c.bool("shutdown", &p.Shutdown) {
		peer.ShutdownReceived = c.time("shutdownT", &p.ShutdownT)
	}
	peer.Encoders = convertList(c, "enc", &p.Enc, (*wirePeerUnit).toPeerUnit)
	peer.Decoders = convertList(c, "dec", &p.Dec, (*wirePeerUnit).toPeerUnit)
	peer.Interfaces = convertList(c, "nets", &p.Network.Nets, (*wireInterface).toInterface)
	return peer
}

// convertList converts every element of a keyed list, scoping issues by list
// name and element key.
func convertList[T any, P keyedElement[T], R any](c *collector, name string, l *keyedList[T, P], convert func(P, *collector) R) []R {
	return convertWhere(c, name, l, nil, convert)
}

// convertWhere converts the elements of a keyed list that match, or all
// elements if match is nil. List level issues are reported by the first call.
func convertWhere[T any, P keyedElement[T], R any](c *collector, name string, l *keyedList[T, P], match func(P) bool, convert func(P, *collector) R) []R {
	lc := c.scope(name)
	lc.add(l.takeIssues()...)
	out := make([]R, 0, len(l.items))
	for _, item := range l.items {
		if match != nil && !match(P(item)) {
			continue
		}
		ec := lc.scope(P(item).key())
		out = append(out, convert(item, ec))
		lc.merge(ec)
	}
	c.merge(lc)
	return out
}

// toSystemInfo converts the fields of the local system that do not belong to
// one of its elements.
func (p *wireParams) toSystemInfo(c *collector) SystemInfo {
	l := &p.Data.Local
	info := SystemInfo{
		ID:              SystemID(l.SysID),
		Name:            l.Name,
		Version:         l.SysVer,
		ST2110:          c.bool("st2110", &l.ST2110),
		NeedsReboot:     c.bool("needReboot", &l.NeedReboot),
		CloudRegistered: c.bool("cloudXlink", &l.CloudXLink),
		Profile:         Profile{ID: l.Profile, Name: l.ProfileName},
		Ports: Ports{
			SystemStatic: c.bool("ports.sysOn", &l.Ports.SysOn),
			SystemPort:   c.int("ports.sysPort", &l.Ports.SysPort),
			DataStatic:   c.bool("ports.portsOn", &l.Ports.PortsOn),
			DataFrom:     c.int("ports.portsFrom", &l.Ports.PortsFrom),
			DataTo:       c.int("ports.portsTo", &l.Ports.PortsTo),
		},
		TrunkMTU: c.int("mtuTrunk", &l.MTUTrunk),
	}
	if info.ID == "" {
		info.ID = SystemID(p.SysID)
	}
	return info
}

func (p *wireParams) toSystem(c *collector) System {
	sys := System{SystemInfo: p.toSystemInfo(c)}
	l := &p.Data.Local
	sys.Encoders = convertList(c, "enc", &l.Enc, (*wireEncoder).toEncoder)
	sys.Decoders = convertList(c, "dec", &l.Dec, (*wireDecoder).toDecoder)
	sys.SRTEncoders = convertWhere(c, "srt", &l.SRT, ofType(UnitSRTEncoder), (*wireMediaUnit).toSRTEncoder)
	sys.SRTDecoders = convertWhere(c, "srt", &l.SRT, ofType(UnitSRTDecoder), (*wireMediaUnit).toSRTDecoder)
	sys.NDIEncoders = convertWhere(c, "ndi", &l.NDI, ofType(UnitNDIEncoder), (*wireMediaUnit).toNDIEncoder)
	sys.NDIDecoders = convertWhere(c, "ndi", &l.NDI, ofType(UnitNDIDecoder), (*wireMediaUnit).toNDIDecoder)
	sys.Trunks = convertList(c, "l2s", &l.L2S, (*wireTrunk).toTrunk)
	sys.Interfaces = convertList(c, "nets", &l.Network.Nets, (*wireInterface).toInterface)
	sys.Peers = convertList(c, "remote", &p.Data.Remote, (*wirePeer).toPeer)
	return sys
}

// errNoState is returned when an update arrives before the full state.
var errNoState = errors.New("xlinkclient: update received before full state")

// state is the merged wire state of one system.
type state struct {
	wire    wireParams
	loaded  bool
	configs configs
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

// errStaleUpdate is returned for an update that is not newer than the state,
// e.g. one that was processed after a later systems.full.
var errStaleUpdate = errors.New("xlinkclient: stale update ignored")

// applyUpdate merges a systems.update message into the state. gap reports
// whether messages were lost since the previous one, judged by dataid. An
// update whose dataid is not newer than the state is ignored and reported as
// errStaleUpdate.
func (s *state) applyUpdate(params json.RawMessage) (gap bool, err error) {
	if !s.loaded {
		return false, errNoState
	}
	var head struct {
		DataID flexInt `json:"dataid"`
	}
	if err := json.Unmarshal(params, &head); err != nil {
		return false, fmt.Errorf("xlinkclient: decode system message: %w", err)
	}
	prev := s.wire.DataID.Value
	if head.DataID.Value <= prev {
		return false, errStaleUpdate
	}
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
	s.configs.attach(&c, &sys)
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
