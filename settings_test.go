package xlinkclient

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// wireValues returns the "values" object as the device receives it.
func wireValues[S ~func(*fields)](t *testing.T, settings ...S) map[string]any {
	t.Helper()
	data, err := json.Marshal(collect(settings).object())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// mustJSON decodes an expected "values" object.
func mustJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEncoderSettingsWireFormat(t *testing.T) {
	got := wireValues(t,
		EncoderBitrate(25),
		EncoderCodec(VideoCodecH265),
		EncoderCRF(false, 17),
		EncoderCard(VideoCardST2110),
		EncoderVideo2110(Stream{Interface: "eth6", Enabled: true, Address: "239.0.0.1", Port: 30000}),
		EncoderAudio2110(Stream{}),
		EncoderVideo2110Payload(96),
		EncoderAudio2110Channels(16),
		EncoderReceiver(""),
		EncoderUplink(""),
		EncoderResendDelay(20*time.Millisecond),
	)
	// The web UI sends ports, payload IDs, SDP channels and the video card as
	// strings, but bitrates and enums as numbers.
	want := mustJSON(t, `{
		"vTBR": 25, "vCodec": 2, "vCRFOn": false, "vCRF": 17, "vCard": "12",
		"v2110NetPri": "eth6", "v2110NetPriEnabled": true,
		"v2110NetPriIp": "239.0.0.1", "v2110NetPriPort": "30000",
		"a2110NetPri": "none", "a2110NetPriEnabled": false,
		"a2110NetPriIp": "", "a2110NetPriPort": "0",
		"v2110RTPpayload": "96", "a2110SDPaCh": "16",
		"receiver": "none", "ethSoMark": "auto", "diffRtt": 20
	}`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("values =\n%v\nwant\n%v", got, want)
	}
}

func TestDecoderSettingsWireFormat(t *testing.T) {
	got := wireValues(t,
		DecoderSender("X8A1111-E1"),
		DecoderPacketBuffer(100*time.Millisecond),
		DecoderSignalGenDelay(time.Minute),
		DecoderSDILevelA(false),
		DecoderOverlayPosition(40, 60),
	)
	// sdilevelA is sent as a string, the signal generator delay in seconds.
	want := mustJSON(t, `{
		"sender": "X8A1111-E1", "pbuf": 100, "vBnoIn": 60,
		"sdilevelA": "false", "vBnoInX": 40, "vBnoInY": 60
	}`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}
}

func TestSRTSettingsWireFormat(t *testing.T) {
	got := wireValues(t,
		SRTEncoderMode(SRTCaller),
		SRTEncoderAddress("192.0.2.1"),
		SRTEncoderPort(7981),
		SRTEncoderLocalPort(7982),
		SRTEncoderLatency(150*time.Millisecond),
		SRTEncoderEncryption(true),
		SRTEncoderKeyLength(SRTAES256),
	)
	// Ports and latency are strings; "encrytion" is the device's spelling.
	want := mustJSON(t, `{
		"mode": 2, "address": "192.0.2.1", "port": "7981", "localPort": "7982",
		"latency": "150", "encrytion": true, "pbkeylen": 32
	}`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}

	got = wireValues(t, NDIDecoderSDILevelA(true), NDIDecoderCard(VideoCardSDI1))
	want = mustJSON(t, `{"sdilevelA": "true", "vCard": "1"}`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NDI values = %v, want %v", got, want)
	}
}

func TestSystemSettingsWireFormat(t *testing.T) {
	got := wireValues(t, InterfaceEnabled(true),
		InterfaceStaticIP("192.0.2.10", "255.255.255.0", "192.0.2.1", [2]string{"8.8.8.8", ""}))
	want := mustJSON(t, `{
		"enabled": true, "dhcp": false, "ip": "192.0.2.10", "mask": "255.255.255.0",
		"gate": "192.0.2.1", "dns1": "8.8.8.8", "dns2": ""
	}`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("interface values = %v, want %v", got, want)
	}

	// PTP values typed into text fields are strings, switches are booleans.
	got = wireValues(t, PTPEnabled(true), PTPDomain(99), PTPAnnounceInterval(-3), PTPInterface(""))
	want = mustJSON(t, `{"ptp": true, "ptpDomainNumber": "99", "ptpLogAnnounceInterval": "-3", "ptpEth": "none"}`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PTP values = %v, want %v", got, want)
	}
}

func TestTrunkMTUOrder(t *testing.T) {
	// Trunk settings are sent one request per key, so the switch must come
	// before the value.
	f := collect([]TrunkSetting{TrunkMTU(1450)})
	if len(f) != 2 || f[0].key != "l2mtuOn" || f[0].value != true || f[1].key != "l2mtu" || f[1].value != 1450 {
		t.Errorf("TrunkMTU(1450) = %v, want l2mtuOn=true then l2mtu=1450", f)
	}
	f = collect([]TrunkSetting{TrunkMTU(0)})
	if len(f) != 1 || f[0].key != "l2mtuOn" || f[0].value != false {
		t.Errorf("TrunkMTU(0) = %v, want only l2mtuOn=false", f)
	}
}

func TestFieldsObjectLastWins(t *testing.T) {
	got := wireValues(t, EncoderBitrate(10), EncoderBitrate(20))
	if got["vTBR"] != float64(20) {
		t.Errorf("vTBR = %v, want 20", got["vTBR"])
	}
}
