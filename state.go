package xlinkclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// errNoState is returned when an update arrives before the full state.
var errNoState = errors.New("xlinkclient: update received before full state")

// errStaleUpdate is returned for an update that is not newer than the state,
// e.g. one that was processed after a later systems.full.
var errStaleUpdate = errors.New("xlinkclient: stale update ignored")

// state is the merged wire state of one system and the configurations of its
// units. Applying a message reports the elements that changed: only the
// elements a message addresses are converted, before and after it is applied.
type state struct {
	wire    wireParams
	loaded  bool
	configs configs
	// sources watch the element lists of wire. They are registered by
	// applyFull.
	sources []updateSource
}

// applyFull replaces the state with a systems.full message.
func (s *state) applyFull(params json.RawMessage) error {
	var w wireParams
	if err := decodeParams(params, &w); err != nil {
		return err
	}
	s.wire = w
	s.loaded = true
	s.watch()
	return nil
}

// watch registers the watchers on the element lists.
func (s *state) watch() {
	l := &s.wire.Data.Local
	s.sources = []updateSource{
		watch("enc", &l.Enc, kindOf(UpdateEncoder), s.encoderUpdate),
		watch("dec", &l.Dec, kindOf(UpdateDecoder), s.decoderUpdate),
		watch("srt", &l.SRT, mediaKind, s.mediaUpdate),
		watch("ndi", &l.NDI, mediaKind, s.mediaUpdate),
		watch("nets", &l.Network.Nets, kindOf(UpdateInterface), interfaceUpdate),
		watch("l2s", &l.L2S, kindOf(UpdateTrunk), trunkUpdate),
		watch("remote", &s.wire.Data.Remote, kindOf(UpdatePeer), peerUpdate),
	}
}

// applyUpdate merges a systems.update message into the state and returns the
// elements that changed. gap reports whether messages were lost since the
// previous one, judged by dataid. An update whose dataid is not newer than the
// state is ignored and reported as errStaleUpdate.
func (s *state) applyUpdate(c *collector, params json.RawMessage) (updates []Update, gap bool, err error) {
	if !s.loaded {
		return nil, false, errNoState
	}
	dataID, err := peekDataID(params)
	if err != nil {
		return nil, false, err
	}
	prev := s.wire.DataID.Value
	if dataID <= prev {
		return nil, false, errStaleUpdate
	}

	var discard collector
	before := s.wire.toSystemInfo(&discard)
	if err := decodeParams(params, &s.wire); err != nil {
		for _, src := range s.sources {
			src.updates(&discard, nil)
		}
		return nil, false, err
	}
	if info := s.wire.toSystemInfo(c); info != before {
		updates = append(updates, Update{Kind: UpdateSystem, Info: &info})
	}
	for _, src := range s.sources {
		updates = src.updates(c, updates)
	}
	return updates, s.wire.DataID.Value != prev+1, nil
}

// peekDataID returns the dataid of a systems.update message without decoding
// the rest. The device sends it before the data; otherwise the head of the
// message is decoded.
func peekDataID(params json.RawMessage) (int, error) {
	if before, after, found := bytes.Cut(params, []byte(`"dataid"`)); found && !bytes.Contains(before, []byte(`"data":`)) {
		if id, ok := parseDataID(after); ok {
			return id, nil
		}
	}
	var head struct {
		DataID flexInt `json:"dataid"`
	}
	if err := json.Unmarshal(params, &head); err != nil {
		return 0, fmt.Errorf("xlinkclient: decode system message: %w", err)
	}
	return head.DataID.Value, nil
}

// parseDataID parses `: 123` or `: "123"` at the start of b.
func parseDataID(b []byte) (int, bool) {
	b = bytes.TrimLeft(b, " \t\r\n")
	if len(b) == 0 || b[0] != ':' {
		return 0, false
	}
	b = bytes.TrimLeft(b[1:], " \t\r\n\"")
	n, digits := 0, 0
	for ; digits < len(b) && b[digits] >= '0' && b[digits] <= '9'; digits++ {
		n = n*10 + int(b[digits]-'0')
	}
	return n, digits > 0 && digits < 19
}

// applyUnitSubscribe applies a state.subscribe response and returns the
// local units whose configuration changed.
func (s *state) applyUnitSubscribe(c *collector, raw json.RawMessage) ([]Update, error) {
	m, err := decodeConfigMessage(raw)
	if err != nil {
		return nil, err
	}
	if s.configs[UnitID(m.ID)] == nil {
		return nil, nil // no longer subscribed
	}
	deps := s.dependents(UnitID(m.ID))
	if err := s.configs.applySubscribe(&m); err != nil {
		return nil, err
	}
	return s.changedDependents(c, deps), nil
}

// applyUnitUpdate applies a state.update and returns the local units whose
// configuration changed. gap reports whether updates of the unit were lost.
func (s *state) applyUnitUpdate(c *collector, raw json.RawMessage) (updates []Update, gap bool, err error) {
	m, err := decodeConfigMessage(raw)
	if err != nil {
		return nil, false, err
	}
	var deps []dependent
	if len(m.Data.Values) > 0 {
		deps = s.dependents(UnitID(m.ID))
	}
	changed, gap, err := s.configs.applyUpdate(&m, raw)
	if err != nil || !changed {
		return nil, gap, err
	}
	return s.changedDependents(c, deps), gap, nil
}

// dependent is a local unit whose state depends on the configuration of a
// unit: the unit itself or a local unit linked to it.
type dependent struct {
	convert func(*collector) Update
	before  Update
}

// dependents returns the local units depending on the configuration of id,
// converted with their current configuration.
func (s *state) dependents(id UnitID) []dependent {
	var deps []dependent
	add := func(convert func(*collector) Update) {
		var discard collector
		deps = append(deps, dependent{convert: convert, before: convert(&discard)})
	}
	l := &s.wire.Data.Local
	for _, e := range l.Enc.items {
		if UnitID(e.ID) == id || UnitID(e.Receiver.ID) == id {
			add(func(c *collector) Update { return s.encoderUpdate(e, c) })
		}
	}
	for _, d := range l.Dec.items {
		if UnitID(d.ID) == id || UnitID(d.Sender.ID) == id {
			add(func(c *collector) Update { return s.decoderUpdate(d, c) })
		}
	}
	for _, list := range []*keyedList[wireMediaUnit, *wireMediaUnit]{&l.SRT, &l.NDI} {
		if u := list.find(string(id)); u != nil {
			add(func(c *collector) Update { return s.mediaUpdate(u, c) })
		}
	}
	return deps
}

// changedDependents returns the dependents that changed.
func (s *state) changedDependents(c *collector, deps []dependent) []Update {
	var updates []Update
	for _, d := range deps {
		after := d.convert(c)
		if !after.sameElement(&d.before) {
			updates = append(updates, after)
		}
	}
	return updates
}

// The update functions convert one element. c is the scope of the list.

func (s *state) encoderUpdate(e *wireEncoder, c *collector) Update {
	enc := e.toEncoder(c)
	s.configs.attachEncoder(c, &enc)
	return Update{Kind: UpdateEncoder, ID: e.ID, Encoder: &enc}
}

func (s *state) decoderUpdate(d *wireDecoder, c *collector) Update {
	dec := d.toDecoder(c)
	s.configs.attachDecoder(c, &dec)
	return Update{Kind: UpdateDecoder, ID: d.ID, Decoder: &dec}
}

// mediaUpdate converts an element of the "srt" or "ndi" list, telling
// encoders and decoders apart by their ID.
func (s *state) mediaUpdate(u *wireMediaUnit, c *collector) Update {
	switch UnitID(u.ID).Type() {
	case UnitSRTEncoder:
		v := u.toSRTEncoder(c)
		s.configs.attachSRTEncoder(c, &v)
		return Update{Kind: UpdateSRTEncoder, ID: u.ID, SRTEncoder: &v}
	case UnitSRTDecoder:
		v := u.toSRTDecoder(c)
		s.configs.attachSRTDecoder(c, &v)
		return Update{Kind: UpdateSRTDecoder, ID: u.ID, SRTDecoder: &v}
	case UnitNDIEncoder:
		v := u.toNDIEncoder(c)
		s.configs.attachNDIEncoder(c, &v)
		return Update{Kind: UpdateNDIEncoder, ID: u.ID, NDIEncoder: &v}
	case UnitNDIDecoder:
		v := u.toNDIDecoder(c)
		s.configs.attachNDIDecoder(c, &v)
		return Update{Kind: UpdateNDIDecoder, ID: u.ID, NDIDecoder: &v}
	default:
		return Update{}
	}
}

func interfaceUpdate(i *wireInterface, c *collector) Update {
	v := i.toInterface(c)
	return Update{Kind: UpdateInterface, ID: i.ID, Interface: &v}
}

func trunkUpdate(t *wireTrunk, c *collector) Update {
	v := t.toTrunk(c)
	return Update{Kind: UpdateTrunk, ID: t.ID, Trunk: &v}
}

func peerUpdate(p *wirePeer, c *collector) Update {
	v := p.toPeer(c)
	return Update{Kind: UpdatePeer, ID: p.SysID, Peer: &v}
}

// snapshot converts the state to the public model. It returns the issues
// found since the previous conversion.
func (s *state) snapshot() (System, []decodeIssue) {
	var c collector
	sys := s.wire.toSystem(&c)
	s.attachConfigs(&c, &sys)
	return sys, c.issues
}

// attachConfigs adds the loaded configurations to the units of sys.
func (s *state) attachConfigs(c *collector, sys *System) {
	for i := range sys.Encoders {
		attachScoped(c, string(sys.Encoders[i].ID), &sys.Encoders[i], s.configs.attachEncoder)
	}
	for i := range sys.Decoders {
		attachScoped(c, string(sys.Decoders[i].ID), &sys.Decoders[i], s.configs.attachDecoder)
	}
	for i := range sys.SRTEncoders {
		attachScoped(c, string(sys.SRTEncoders[i].ID), &sys.SRTEncoders[i], s.configs.attachSRTEncoder)
	}
	for i := range sys.SRTDecoders {
		attachScoped(c, string(sys.SRTDecoders[i].ID), &sys.SRTDecoders[i], s.configs.attachSRTDecoder)
	}
	for i := range sys.NDIEncoders {
		attachScoped(c, string(sys.NDIEncoders[i].ID), &sys.NDIEncoders[i], s.configs.attachNDIEncoder)
	}
	for i := range sys.NDIDecoders {
		attachScoped(c, string(sys.NDIDecoders[i].ID), &sys.NDIDecoders[i], s.configs.attachNDIDecoder)
	}
}

// attachScoped attaches the configuration of unit id, scoping issues by id.
func attachScoped[U any](c *collector, id string, u *U, attach func(*collector, *U)) {
	c.push(id)
	attach(c, u)
	c.pop()
}
