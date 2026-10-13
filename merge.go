package xlinkclient

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
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
	// observe, if set, is called for every element of a decoded array
	// before it is applied, with the existing element or nil for a new one.
	observe func(key string, existing P, deleted bool)
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
		if l.observe != nil {
			var existing P
			if idx >= 0 {
				existing = l.items[idx]
			}
			l.observe(k, existing, head.Delete)
		}

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

// find returns the element with the given key, or nil.
func (l *keyedList[T, P]) find(key string) P {
	for _, e := range l.items {
		if P(e).key() == key {
			return e
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
		if typeErr.Field != "" {
			key += "." + typeErr.Field
		}
		return decodeIssue{Key: key, Reason: "unexpected " + typeErr.Value}
	}
	return decodeIssue{Key: key, Raw: raw, Reason: err.Error()}
}

// collector converts lenient wire values to plain values and gathers every
// invalid one as a decodeIssue. Each invalid value is reported only once: its
// marker is cleared after it has been collected. Issue keys are prefixed with
// the path of the element being converted; push and pop maintain it without
// allocating, the key is only built for an issue.
type collector struct {
	path   []string
	issues []decodeIssue
}

// push enters a nested scope such as a list, a unit ID or "values".
func (c *collector) push(name string) {
	c.path = append(c.path, name)
}

// pop leaves the scope entered by the matching push.
func (c *collector) pop() {
	c.path = c.path[:len(c.path)-1]
}

func (c *collector) add(issues ...decodeIssue) {
	for _, i := range issues {
		if len(c.path) > 0 {
			i.Key = strings.Join(c.path, ".") + "." + i.Key
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

func (c *collector) float(key string, f *flexFloat) float64 {
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
