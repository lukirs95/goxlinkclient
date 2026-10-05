package xlinkclient

import (
	"strconv"
	"strings"
)

// SystemID identifies a VideoXLink system, e.g. "X8A1111". The second
// character is the hardware generation ('8' for X8, '4' for X4).
type SystemID string

// UnitID identifies a video unit on a system, e.g. "X8A1111-E1" for the first
// XLink encoder or "X8A1111-srtD2" for the second SRT decoder.
type UnitID string

// TrunkID identifies an XLink layer 2 trunk on a system, e.g. "X8A1111-L2S1".
type TrunkID string

// UnitType is the kind of a video unit. The values match "stateType" in
// state.subscribe and "type" in newVideo and systems.full.
type UnitType int

const (
	UnitXLinkEncoder UnitType = 1
	UnitXLinkDecoder UnitType = 2
	UnitNDIEncoder   UnitType = 3
	UnitNDIDecoder   UnitType = 4
	UnitSRTEncoder   UnitType = 8
	UnitSRTDecoder   UnitType = 9
)

func (t UnitType) String() string {
	switch t {
	case UnitXLinkEncoder:
		return "XLink encoder"
	case UnitXLinkDecoder:
		return "XLink decoder"
	case UnitNDIEncoder:
		return "NDI encoder"
	case UnitNDIDecoder:
		return "NDI decoder"
	case UnitSRTEncoder:
		return "SRT encoder"
	case UnitSRTDecoder:
		return "SRT decoder"
	default:
		return "UnitType(" + strconv.Itoa(int(t)) + ")"
	}
}

// IsEncoder reports whether units of this type send video.
func (t UnitType) IsEncoder() bool {
	return t == UnitXLinkEncoder || t == UnitNDIEncoder || t == UnitSRTEncoder
}

// unitPrefixes maps the type part of a UnitID to its UnitType. Longer prefixes
// come first so "NdiE" is not mistaken for "E".
var unitPrefixes = []struct {
	prefix string
	typ    UnitType
}{
	{"NdiE", UnitNDIEncoder},
	{"NdiD", UnitNDIDecoder},
	{"srtE", UnitSRTEncoder},
	{"srtD", UnitSRTDecoder},
	{"E", UnitXLinkEncoder},
	{"D", UnitXLinkDecoder},
}

// HardwareGeneration returns the generation digit of the system, e.g. 8 for
// "X8A1111". ok is false if the ID does not follow that pattern.
func (id SystemID) HardwareGeneration() (gen int, ok bool) {
	if len(id) < 2 || id[0] != 'X' || id[1] < '0' || id[1] > '9' {
		return 0, false
	}
	return int(id[1] - '0'), true
}

// splitID splits "<system>-<suffix>" at the first dash.
func splitID(id string) (system SystemID, suffix string, ok bool) {
	sys, rest, found := strings.Cut(id, "-")
	if !found || sys == "" || rest == "" {
		return "", "", false
	}
	return SystemID(sys), rest, true
}

// parse splits the ID into its parts. ok is false if the ID is malformed.
func (id UnitID) parse() (system SystemID, typ UnitType, index int, ok bool) {
	system, suffix, ok := splitID(string(id))
	if !ok {
		return "", 0, 0, false
	}
	for _, p := range unitPrefixes {
		digits, found := strings.CutPrefix(suffix, p.prefix)
		if !found {
			continue
		}
		n, err := strconv.Atoi(digits)
		if err != nil || n < 1 {
			return "", 0, 0, false
		}
		return system, p.typ, n, true
	}
	return "", 0, 0, false
}

// System returns the system the unit belongs to, or "" if the ID is malformed.
func (id UnitID) System() SystemID {
	system, _, _, _ := id.parse()
	return system
}

// Type returns the kind of the unit, or 0 if the ID is malformed.
func (id UnitID) Type() UnitType {
	_, typ, _, _ := id.parse()
	return typ
}

// Index returns the one based number of the unit within its type, or 0 if the
// ID is malformed.
func (id UnitID) Index() int {
	_, _, index, _ := id.parse()
	return index
}

// Valid reports whether the ID has the form "<system>-<type><index>".
func (id UnitID) Valid() bool {
	_, _, _, ok := id.parse()
	return ok
}

// System returns the system the trunk belongs to, or "" if the ID is
// malformed.
func (id TrunkID) System() SystemID {
	system, suffix, ok := splitID(string(id))
	if !ok || !strings.HasPrefix(suffix, "L2S") {
		return ""
	}
	return system
}
