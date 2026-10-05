package xlinkclient

import (
	"slices"
	"time"
)

// System is a snapshot of the local VideoXLink system. It is built from
// systems.full and kept current by applying systems.update deltas. A System
// returned by the Client is a copy and may be retained and modified freely.
type System struct {
	ID          SystemID
	Name        string
	Version     string
	Beta        bool
	ST2110      bool
	NeedsReboot bool
	StartedAt   time.Time
	// Profile is the running profile.
	Profile Profile
	Ports   Ports
	// TrunkMTU is the default MTU of layer 2 trunks.
	TrunkMTU    int
	Encoders    []Encoder
	Decoders    []Decoder
	SRTEncoders []SRTEncoder
	SRTDecoders []SRTDecoder
	NDIEncoders []NDIEncoder
	NDIDecoders []NDIDecoder
	Interfaces  []Interface
	Trunks     []Trunk
	// Peers are the configured remote systems.
	Peers []Peer
}

// Profile identifies a configuration profile.
type Profile struct {
	ID   string
	Name string
}

// Ports is the XLink port configuration. A static port is used as configured;
// otherwise the device picks one dynamically.
type Ports struct {
	SystemStatic bool
	SystemPort   int
	DataStatic   bool
	DataFrom     int
	DataTo       int
}

// Peer is a configured remote system as seen from the local system.
type Peer struct {
	ID         SystemID
	Name       string
	Version    string
	ST2110     bool
	Connected  bool
	P2P        bool
	OnLocalNet bool
	// IP and Port are the address the connection is established with.
	IP             string
	Port           int
	ConnectedSince time.Time
	LastSeen       time.Time
	ManualIP       ManualIP
	Encoders       []PeerUnit
	Decoders       []PeerUnit
	Interfaces     []Interface
}

// ManualIP is the manual connection setting for a remote system.
type ManualIP struct {
	Primary     string
	Secondary   string
	Port        int
	AutoConnect bool
}

// PeerUnit is the summary of a unit on a remote system.
type PeerUnit struct {
	ID      UnitID
	Name    string
	Enabled bool
	Running bool
	// Linked is the receiver of an encoder or the sender of a decoder, or ""
	// if none is selected.
	Linked UnitID
}

// Encoder returns the local encoder with the given ID.
func (s System) Encoder(id UnitID) (Encoder, bool) {
	return find(s.Encoders, func(e Encoder) bool { return e.ID == id })
}

// Decoder returns the local decoder with the given ID.
func (s System) Decoder(id UnitID) (Decoder, bool) {
	return find(s.Decoders, func(d Decoder) bool { return d.ID == id })
}

// SRTEncoder returns the local SRT encoder with the given ID.
func (s System) SRTEncoder(id UnitID) (SRTEncoder, bool) {
	return find(s.SRTEncoders, func(e SRTEncoder) bool { return e.ID == id })
}

// SRTDecoder returns the local SRT decoder with the given ID.
func (s System) SRTDecoder(id UnitID) (SRTDecoder, bool) {
	return find(s.SRTDecoders, func(d SRTDecoder) bool { return d.ID == id })
}

// NDIEncoder returns the local NDI encoder with the given ID.
func (s System) NDIEncoder(id UnitID) (NDIEncoder, bool) {
	return find(s.NDIEncoders, func(e NDIEncoder) bool { return e.ID == id })
}

// NDIDecoder returns the local NDI decoder with the given ID.
func (s System) NDIDecoder(id UnitID) (NDIDecoder, bool) {
	return find(s.NDIDecoders, func(d NDIDecoder) bool { return d.ID == id })
}

// Interface returns the network interface with the given name, e.g. "eth0".
func (s System) Interface(id string) (Interface, bool) {
	return find(s.Interfaces, func(i Interface) bool { return i.ID == id })
}

// Trunk returns the layer 2 trunk with the given ID.
func (s System) Trunk(id TrunkID) (Trunk, bool) {
	return find(s.Trunks, func(t Trunk) bool { return t.ID == id })
}

// Peer returns the remote system with the given ID.
func (s System) Peer(id SystemID) (Peer, bool) {
	return find(s.Peers, func(p Peer) bool { return p.ID == id })
}

func find[T any](items []T, match func(T) bool) (T, bool) {
	if i := slices.IndexFunc(items, match); i >= 0 {
		return items[i], true
	}
	var zero T
	return zero, false
}
