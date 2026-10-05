package xlinkclient

import "testing"

func TestUnitIDParse(t *testing.T) {
	tests := []struct {
		id         UnitID
		wantSystem SystemID
		wantType   UnitType
		wantIndex  int
		wantValid  bool
	}{
		{id: "X8A1111-E1", wantSystem: "X8A1111", wantType: UnitXLinkEncoder, wantIndex: 1, wantValid: true},
		{id: "X8A1111-D6", wantSystem: "X8A1111", wantType: UnitXLinkDecoder, wantIndex: 6, wantValid: true},
		{id: "X8A1111-NdiE7", wantSystem: "X8A1111", wantType: UnitNDIEncoder, wantIndex: 7, wantValid: true},
		{id: "X8A1111-NdiD7", wantSystem: "X8A1111", wantType: UnitNDIDecoder, wantIndex: 7, wantValid: true},
		{id: "X8A1111-srtE1", wantSystem: "X8A1111", wantType: UnitSRTEncoder, wantIndex: 1, wantValid: true},
		{id: "X8A1111-srtD12", wantSystem: "X8A1111", wantType: UnitSRTDecoder, wantIndex: 12, wantValid: true},
		{id: "X4A2222-E5", wantSystem: "X4A2222", wantType: UnitXLinkEncoder, wantIndex: 5, wantValid: true},
		// The device uses "none" for an unset sender or receiver.
		{id: "none"},
		{id: ""},
		{id: "X8A1111"},
		{id: "X8A1111-"},
		{id: "-E1"},
		{id: "X8A1111-E"},
		{id: "X8A1111-E0"},
		{id: "X8A1111-Ex"},
		{id: "X8A1111-L2S1"},
		{id: "eth0"},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			if got := tt.id.Valid(); got != tt.wantValid {
				t.Errorf("Valid() = %t, want %t", got, tt.wantValid)
			}
			if got := tt.id.System(); got != tt.wantSystem {
				t.Errorf("System() = %q, want %q", got, tt.wantSystem)
			}
			if got := tt.id.Type(); got != tt.wantType {
				t.Errorf("Type() = %v, want %v", got, tt.wantType)
			}
			if got := tt.id.Index(); got != tt.wantIndex {
				t.Errorf("Index() = %d, want %d", got, tt.wantIndex)
			}
		})
	}
}

func TestTrunkIDSystem(t *testing.T) {
	tests := []struct {
		id   TrunkID
		want SystemID
	}{
		{id: "X8A1111-L2S1", want: "X8A1111"},
		{id: "X8A1111-E1", want: ""},
		{id: "local", want: ""},
		{id: "", want: ""},
	}
	for _, tt := range tests {
		if got := tt.id.System(); got != tt.want {
			t.Errorf("TrunkID(%q).System() = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestSystemIDHardwareGeneration(t *testing.T) {
	tests := []struct {
		id     SystemID
		want   int
		wantOK bool
	}{
		{id: "X8A1111", want: 8, wantOK: true},
		{id: "X4A2222", want: 4, wantOK: true},
		{id: "X", wantOK: false},
		{id: "Y8A1111", wantOK: false},
		{id: "", wantOK: false},
	}
	for _, tt := range tests {
		got, ok := tt.id.HardwareGeneration()
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("SystemID(%q).HardwareGeneration() = %d, %t, want %d, %t", tt.id, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestUnitTypeIsEncoder(t *testing.T) {
	for _, typ := range []UnitType{UnitXLinkEncoder, UnitNDIEncoder, UnitSRTEncoder} {
		if !typ.IsEncoder() {
			t.Errorf("%v.IsEncoder() = false, want true", typ)
		}
	}
	for _, typ := range []UnitType{UnitXLinkDecoder, UnitNDIDecoder, UnitSRTDecoder, 0} {
		if typ.IsEncoder() {
			t.Errorf("%v.IsEncoder() = true, want false", typ)
		}
	}
}

func TestVideoCardSDIPort(t *testing.T) {
	tests := []struct {
		card     VideoCard
		wantPort int
		wantOK   bool
		wantStr  string
	}{
		{card: VideoCardNone, wantStr: "None"},
		{card: VideoCardSDI1, wantPort: 1, wantOK: true, wantStr: "SDI (1)"},
		{card: VideoCardSDI8, wantPort: 8, wantOK: true, wantStr: "SDI (8)"},
		{card: VideoCardNDI, wantStr: "NDI"},
		{card: VideoCardST2110, wantStr: "2110"},
		{card: 9, wantStr: "VideoCard(9)"},
	}
	for _, tt := range tests {
		port, ok := tt.card.SDIPort()
		if port != tt.wantPort || ok != tt.wantOK {
			t.Errorf("%d.SDIPort() = %d, %t, want %d, %t", tt.card, port, ok, tt.wantPort, tt.wantOK)
		}
		if s := tt.card.String(); s != tt.wantStr {
			t.Errorf("%d.String() = %q, want %q", tt.card, s, tt.wantStr)
		}
	}
}
