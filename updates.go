package xlinkclient

import (
	"slices"
	"strconv"
)

// UpdateKind identifies what an Update carries. It is meant as the key for
// dispatching updates to specific handlers.
type UpdateKind int

const (
	// UpdateSnapshot carries the complete system. It is sent when the
	// connection is established and replaces all previous state.
	UpdateSnapshot UpdateKind = iota + 1
	// UpdateSystem carries the fields of the system that do not belong to
	// one of its elements.
	UpdateSystem
	UpdateEncoder
	UpdateDecoder
	UpdateSRTEncoder
	UpdateSRTDecoder
	UpdateNDIEncoder
	UpdateNDIDecoder
	UpdateInterface
	UpdateTrunk
	UpdatePeer
)

func (k UpdateKind) String() string {
	switch k {
	case UpdateSnapshot:
		return "Snapshot"
	case UpdateSystem:
		return "System"
	case UpdateEncoder:
		return "Encoder"
	case UpdateDecoder:
		return "Decoder"
	case UpdateSRTEncoder:
		return "SRTEncoder"
	case UpdateSRTDecoder:
		return "SRTDecoder"
	case UpdateNDIEncoder:
		return "NDIEncoder"
	case UpdateNDIDecoder:
		return "NDIDecoder"
	case UpdateInterface:
		return "Interface"
	case UpdateTrunk:
		return "Trunk"
	case UpdatePeer:
		return "Peer"
	default:
		return "UpdateKind(" + strconv.Itoa(int(k)) + ")"
	}
}

// Update reports the complete system or one element of it that was added,
// changed or removed. The field matching Kind holds the complete new state;
// all of them are nil if the element was removed. An Update is a copy and may
// be retained and modified freely.
type Update struct {
	Client *Client
	System SystemID
	Kind   UpdateKind
	// ID identifies the element: a unit, trunk or system ID or an interface
	// name. It is empty for UpdateSnapshot and UpdateSystem.
	ID      string
	Removed bool

	Snapshot   *System
	Info       *SystemInfo
	Encoder    *Encoder
	Decoder    *Decoder
	SRTEncoder *SRTEncoder
	SRTDecoder *SRTDecoder
	NDIEncoder *NDIEncoder
	NDIDecoder *NDIDecoder
	Interface  *Interface
	Trunk      *Trunk
	Peer       *Peer
}

// sameElement reports whether u and v, which are of the same kind, carry
// equal elements.
func (u *Update) sameElement(v *Update) bool {
	switch u.Kind {
	case UpdateSystem:
		return equalPtr(u.Info, v.Info)
	case UpdateEncoder:
		return u.Encoder != nil && v.Encoder != nil && u.Encoder.equal(v.Encoder)
	case UpdateDecoder:
		return u.Decoder != nil && v.Decoder != nil && u.Decoder.equal(v.Decoder)
	case UpdateSRTEncoder:
		return u.SRTEncoder != nil && v.SRTEncoder != nil && u.SRTEncoder.equal(v.SRTEncoder)
	case UpdateSRTDecoder:
		return u.SRTDecoder != nil && v.SRTDecoder != nil && u.SRTDecoder.equal(v.SRTDecoder)
	case UpdateNDIEncoder:
		return u.NDIEncoder != nil && v.NDIEncoder != nil && u.NDIEncoder.equal(v.NDIEncoder)
	case UpdateNDIDecoder:
		return u.NDIDecoder != nil && v.NDIDecoder != nil && u.NDIDecoder.equal(v.NDIDecoder)
	case UpdateInterface:
		return equalPtr(u.Interface, v.Interface)
	case UpdateTrunk:
		return equalPtr(u.Trunk, v.Trunk)
	case UpdatePeer:
		return u.Peer != nil && v.Peer != nil && u.Peer.equal(v.Peer)
	default:
		return false
	}
}

// equalPtr reports whether a and b are both nil or point to equal values.
func equalPtr[T comparable](a, b *T) bool {
	return a == b || a != nil && b != nil && *a == *b
}

// The equal methods compare by value, including the configurations the
// elements point to.

func (e *Encoder) equal(o *Encoder) bool {
	if !equalPtr(e.Config, o.Config) || !equalPtr(e.Receiver.Config, o.Receiver.Config) {
		return false
	}
	a, b := *e, *o
	a.Config, b.Config = nil, nil
	a.Receiver.Config, b.Receiver.Config = nil, nil
	return a == b
}

func (d *Decoder) equal(o *Decoder) bool {
	if !equalPtr(d.Config, o.Config) || !equalPtr(d.Sender.Config, o.Sender.Config) {
		return false
	}
	a, b := *d, *o
	a.Config, b.Config = nil, nil
	a.Sender.Config, b.Sender.Config = nil, nil
	return a == b
}

func (e *SRTEncoder) equal(o *SRTEncoder) bool {
	a, b := *e, *o
	a.Config, b.Config = nil, nil
	return a == b && equalPtr(e.Config, o.Config)
}

func (d *SRTDecoder) equal(o *SRTDecoder) bool {
	a, b := *d, *o
	a.Config, b.Config = nil, nil
	return a == b && equalPtr(d.Config, o.Config)
}

func (e *NDIEncoder) equal(o *NDIEncoder) bool {
	a, b := *e, *o
	a.Config, b.Config = nil, nil
	return a == b && equalPtr(e.Config, o.Config)
}

func (d *NDIDecoder) equal(o *NDIDecoder) bool {
	a, b := *d, *o
	a.Config, b.Config = nil, nil
	return a == b && equalPtr(d.Config, o.Config)
}

func (p *Peer) equal(o *Peer) bool {
	if !slices.Equal(p.Encoders, o.Encoders) || !slices.Equal(p.Decoders, o.Decoders) ||
		!slices.Equal(p.Interfaces, o.Interfaces) {
		return false
	}
	return p.PeerInfo == o.PeerInfo
}

// touched is an element addressed by a decoded delta.
type touched struct {
	key     string
	existed bool
	deleted bool
	// before is the element before the delta was applied, if it existed.
	before Update
}

// watcher records the elements of a keyed list that a delta touches and turns
// the ones that changed into updates. Only touched elements are converted:
// before the delta is applied and afterwards.
type watcher[T any, P keyedElement[T]] struct {
	name    string
	list    *keyedList[T, P]
	kind    func(key string) UpdateKind
	convert func(P, *collector) Update
	touched []touched
}

// watch registers a watcher on l.
func watch[T any, P keyedElement[T]](name string, l *keyedList[T, P], kind func(string) UpdateKind,
	convert func(P, *collector) Update) *watcher[T, P] {
	w := &watcher[T, P]{name: name, list: l, kind: kind, convert: convert}
	l.observe = w.observe
	return w
}

func (w *watcher[T, P]) observe(key string, existing P, deleted bool) {
	for i := range w.touched {
		if w.touched[i].key == key {
			w.touched[i].deleted = deleted
			return
		}
	}
	t := touched{key: key, existed: existing != nil, deleted: deleted}
	if existing != nil && !deleted {
		// Issues of the previous values were reported when they arrived.
		var discard collector
		t.before = w.convert(existing, &discard)
	}
	w.touched = append(w.touched, t)
}

// updates appends an update for every touched element that was added,
// changed or removed, and resets the watcher.
func (w *watcher[T, P]) updates(c *collector, out []Update) []Update {
	lc := c.scope(w.name)
	lc.add(w.list.takeIssues()...)
	for _, t := range w.touched {
		if t.deleted {
			if k := w.kind(t.key); t.existed && k != 0 {
				out = append(out, Update{Kind: k, ID: t.key, Removed: true})
			}
			continue
		}
		e := w.list.find(t.key)
		if e == nil {
			continue
		}
		ec := lc.scope(t.key)
		after := w.convert(e, ec)
		lc.merge(ec)
		if after.Kind == 0 || t.existed && after.sameElement(&t.before) {
			continue
		}
		out = append(out, after)
	}
	w.touched = w.touched[:0]
	c.merge(lc)
	return out
}

// updateSource is a watcher of any element type.
type updateSource interface {
	updates(c *collector, out []Update) []Update
}

func kindOf(k UpdateKind) func(string) UpdateKind {
	return func(string) UpdateKind { return k }
}

// mediaKind returns the kind of an element of the "srt" and "ndi" lists,
// which mix encoders and decoders, from its ID.
func mediaKind(key string) UpdateKind {
	switch UnitID(key).Type() {
	case UnitSRTEncoder:
		return UpdateSRTEncoder
	case UnitSRTDecoder:
		return UpdateSRTDecoder
	case UnitNDIEncoder:
		return UpdateNDIEncoder
	case UnitNDIDecoder:
		return UpdateNDIDecoder
	default:
		return 0
	}
}
