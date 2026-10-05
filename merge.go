package xlinkclient

import (
	"encoding/json"
	"errors"
	"slices"
	"time"
)

// systems.full carries the complete state of a system, systems.update only the
// fields that changed. Arrays in an update are not addressed by index but by a
// key ("id" for units, interfaces and trunks, "sysid" for remote systems):
//
//   - an element with an unknown key is new and is sent completely,
//   - an element with a known key carries only the changed fields,
//   - an element with "delete": true has been removed.
//
// The wire types decode messages into a persistent state using these rules.
// Scalars need no special handling: encoding/json only assigns fields present
// in the input, so decoding a delta into the existing state merges it.

// keyedElement is implemented by pointers to wire elements of keyed arrays.
type keyedElement[T any] interface {
	*T
	key() string
}

// keyedList is a JSON array that is merged by key when decoded into an
// existing value. Problems with single elements are recorded instead of
// failing the whole message.
type keyedList[T any, P keyedElement[T]] struct {
	items  []*T
	issues []decodeIssue
}

// keyedHead holds the fields needed to address an element before decoding it.
type keyedHead struct {
	ID     string `json:"id"`
	SysID  string `json:"sysid"`
	Delete bool   `json:"delete"`
}

func (h keyedHead) key() string {
	if h.ID != "" {
		return h.ID
	}
	return h.SysID
}

// UnmarshalJSON implements json.Unmarshaler. It only fails if data is not an
// array; problems with single elements are recorded as issues.
func (l *keyedList[T, P]) UnmarshalJSON(data []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}

	for _, raw := range raws {
		var head keyedHead
		if err := json.Unmarshal(raw, &head); err != nil || head.key() == "" {
			l.issues = append(l.issues, decodeIssue{Key: "element without key", Raw: raw})
			continue
		}
		k := head.key()
		idx := slices.IndexFunc(l.items, func(e *T) bool { return P(e).key() == k })

		if head.Delete {
			if idx >= 0 {
				l.items = slices.Delete(l.items, idx, idx+1)
			}
			continue
		}

		if idx < 0 {
			l.items = append(l.items, new(T))
			idx = len(l.items) - 1
		}
		if err := json.Unmarshal(raw, l.items[idx]); err != nil {
			l.issues = append(l.issues, issueFromError(k, raw, err))
		}
	}
	return nil
}

// takeIssues returns and clears the issues recorded while decoding.
func (l *keyedList[T, P]) takeIssues() []decodeIssue {
	issues := l.issues
	l.issues = nil
	return issues
}

// issueFromError turns a decoding error into a decodeIssue. Type mismatches
// name the offending field; any other error reports the whole input.
func issueFromError(key string, raw json.RawMessage, err error) decodeIssue {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return decodeIssue{Key: key + "." + typeErr.Field, Reason: "unexpected " + typeErr.Value}
	}
	return decodeIssue{Key: key, Raw: raw, Reason: err.Error()}
}

// collector converts lenient wire values to plain values and gathers every
// invalid one as a decodeIssue. Each invalid value is reported only once: its
// marker is cleared after it has been collected.
type collector struct {
	prefix string
	issues []decodeIssue
}

// scope returns a collector that prefixes keys with name, e.g. a unit ID.
func (c *collector) scope(name string) *collector {
	prefix := name
	if c.prefix != "" {
		prefix = c.prefix + "." + name
	}
	return &collector{prefix: prefix}
}

// merge appends the issues of a scoped collector.
func (c *collector) merge(other *collector) {
	c.issues = append(c.issues, other.issues...)
}

func (c *collector) add(issues ...decodeIssue) {
	for _, i := range issues {
		if c.prefix != "" {
			i.Key = c.prefix + "." + i.Key
		}
		c.issues = append(c.issues, i)
	}
}

func (c *collector) int(key string, f *flexInt) int {
	if issue, ok := f.issue(key); ok {
		c.add(issue)
		f.Invalid = nil
	}
	return f.Value
}

func (c *collector) bool(key string, f *flexBool) bool {
	if issue, ok := f.issue(key); ok {
		c.add(issue)
		f.Invalid = nil
	}
	return f.Value
}

func (c *collector) time(key string, f *flexTime) time.Time {
	if issue, ok := f.issue(key); ok {
		c.add(issue)
		f.Invalid = nil
	}
	return f.Value
}

// unitID converts a unit reference, mapping the device's "none" to "".
func unitID(s string) UnitID {
	if s == "none" {
		return ""
	}
	return UnitID(s)
}

// iface converts an interface reference, mapping the device's "none" to "".
func iface(s string) string {
	if s == "none" {
		return ""
	}
	return s
}
