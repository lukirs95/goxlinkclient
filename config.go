package xlinkclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// The configuration of a unit is not part of systems.full. The web UI loads it
// with state.subscribe when a settings dialog is opened; the device then
// pushes every change as state.update until state.unsubscribe. The client
// subscribes the local units and their counterparts and keeps their
// configuration in the snapshot. The fields mirror the settings of the
// Configure methods.

// EncoderConfig is the configuration of an XLink encoder.
type EncoderConfig struct {
	Name         string
	AutoStart    bool
	BarsStandard VideoMode
	NoSignal     NoSignalMode
	Codec        VideoCodec
	BitDepth     int
	// Chroma is the chroma subsampling, 420 or 422.
	Chroma       int
	IntraRefresh bool
	Interlaced   bool
	// CRF is the constant rate factor, used if CRFOn is set.
	CRFOn bool
	CRF   int
	// GOP is the GOP size, used if GOPOn is set.
	GOPOn    bool
	GOP      int
	FECLevel int
	// Bitrate is the target video bitrate in Mbps.
	Bitrate       int
	AudioMode     AudioMode
	AudioBitDepth int
	AudioChannels int
	AudioFEC      bool
	// AudioBitrate is the compressed audio bitrate per channel in kbps.
	AudioBitrate int
	Card         VideoCard
	VideoMode    VideoMode
	// VideoLock is the locked video standard of a 2110 input, VideoModeAuto
	// if it is not locked.
	VideoLock           VideoMode
	Video               bool
	Audio               bool
	Video2110           Stream
	Audio2110           Stream
	Video2110Payload    int
	Audio2110Payload    int
	Video2110Source     string
	Audio2110Source     string
	Audio2110Channels   int
	Audio2110PacketTime int
	PacketAck           bool
	Resend              bool
	MaxResends          int
	ResendDelay         time.Duration
	MaxRTT              time.Duration
	// Uplink is the interface the XLink traffic is forced to, or "" if it is
	// selected automatically.
	Uplink string
}

// DecoderConfig is the configuration of an XLink decoder.
type DecoderConfig struct {
	Name      string
	AutoStart bool
	// Sender is the encoder the decoder receives from, or "" if none is
	// selected.
	Sender          UnitID
	AVSync          bool
	FPSSync         bool
	PacketBuffer    time.Duration
	AllowLateFrames bool
	SignalGen       bool
	SignalGenDelay  time.Duration
	SignalGenType   SignalGenerator
	// OverlayText is the template of the info overlay, shown if
	// OverlayTextOn is set.
	OverlayTextOn           bool
	OverlayText             string
	OverlaySize             int
	OverlayX                int
	OverlayY                int
	Card                    VideoCard
	VideoMode               VideoMode
	SDILevelA               bool
	ProgressiveNotPsF       bool
	Video2110               Stream
	Audio2110               Stream
	Video2110Payload        int
	Audio2110Payload        int
	Video2110StopOnNoSignal bool
	Audio2110StopOnNoSignal bool
	Audio2110PacketTime     int
	// Uplink is the interface the XLink traffic is forced to, or "" if it is
	// selected automatically.
	Uplink string
}

// SRTEncoderConfig is the configuration of an SRT encoder.
type SRTEncoderConfig struct {
	Name      string
	AutoStart bool
	Mode      SRTMode
	// Address and Port are the remote endpoint in caller mode. LocalPort is
	// the port to listen on in listener mode, or the source port in caller
	// mode if FixedSourcePort is set.
	Address         string
	Port            int
	LocalPort       int
	FixedSourcePort bool
	Latency         time.Duration
	Encryption      bool
	KeyLength       SRTKeyLength
	Passphrase      string
	BitDepth        int
	IntraRefresh    bool
	// GOP is the GOP size, used if GOPOn is set.
	GOPOn bool
	GOP   int
	// Bitrate is the video bitrate in Mbps, AudioBitrate the audio bitrate
	// per channel in kbps.
	Bitrate       int
	AudioBitrate  int
	Card          VideoCard
	VideoMode     VideoMode
	AudioChannels int
}

// SRTDecoderConfig is the configuration of an SRT decoder.
type SRTDecoderConfig struct {
	Name      string
	AutoStart bool
	Mode      SRTMode
	// Address and Port are the remote endpoint in caller mode. LocalPort is
	// the port to listen on in listener mode, or the destination port in
	// caller mode if FixedLocalPort is set.
	Address        string
	Port           int
	LocalPort      int
	FixedLocalPort bool
	Latency        time.Duration
	Encryption     bool
	KeyLength      SRTKeyLength
	Passphrase     string
	Card           VideoCard
	Audio          bool
	AudioCodec     SRTAudioCodec
}

// NDIEncoderConfig is the configuration of an NDI encoder.
type NDIEncoderConfig struct {
	Name          string
	AutoStart     bool
	Card          VideoCard
	VideoMode     VideoMode
	BarsStandard  VideoMode
	AudioChannels int
}

// NDIDecoderConfig is the configuration of an NDI decoder.
type NDIDecoderConfig struct {
	Name              string
	AutoStart         bool
	AVSync            bool
	FPSSync           bool
	SDIBuffer         bool
	AudioChannels     int
	Card              VideoCard
	VideoMode         VideoMode
	SDILevelA         bool
	ProgressiveNotPsF bool
	Resize            bool
}

// wireConfigValues holds the "values" keys of all unit kinds, as sent by
// state.subscribe and state.update.
type wireConfigValues struct {
	Name         string   `json:"name"`
	AutoStart    flexBool `json:"autoStart"`
	VModeB       string   `json:"vModeB"`
	VNoS         flexInt  `json:"vNoS"`
	VCodec       flexInt  `json:"vCodec"`
	VBit         flexInt  `json:"vBit"`
	VColor       flexInt  `json:"vColor"`
	VPIntraOn    flexBool `json:"vPIntraOn"`
	VIENC        flexBool `json:"vIENC"`
	VCRFOn       flexBool `json:"vCRFOn"`
	VCRF         flexInt  `json:"vCRF"`
	VGOPOn       flexBool `json:"vGOPOn"`
	VGOP         flexInt  `json:"vGOP"`
	VFEC         flexInt  `json:"vFEC"`
	VTBR         flexInt  `json:"vTBR"`
	AMode        flexInt  `json:"aMode"`
	ABit         flexInt  `json:"aBit"`
	ACh          flexInt  `json:"aCh"`
	AFEC         flexBool `json:"aFEC"`
	ATBR         flexInt  `json:"aTBR"`
	VCard        flexInt  `json:"vCard"`
	VMode        string   `json:"vMode"`
	VModeLock    string   `json:"vModeLock"`
	Video        flexBool `json:"video"`
	Audio        flexBool `json:"audio"`
	XAck         flexBool `json:"xAck"`
	XAckR        flexBool `json:"xAckR"`
	XAckRmax     flexInt  `json:"xAckRmax"`
	DiffRtt      flexInt  `json:"diffRtt"`
	MaxRtt       flexInt  `json:"maxRtt"`
	EthSoMark    string   `json:"ethSoMark"`
	Sender       string   `json:"sender"`
	VAVSOn       flexBool `json:"vAVSOn"`
	VFRCOn       flexBool `json:"vFRCOn"`
	PBuf         flexInt  `json:"pbuf"`
	Late         flexBool `json:"late"`
	VBnoInOn     flexBool `json:"vBnoInOn"`
	VBnoIn       flexInt  `json:"vBnoIn"`
	VBnoInType   string   `json:"vBnoInType"`
	VBnoInTextOn flexBool `json:"vBnoInTextOn"`
	VBnoInText   string   `json:"vBnoInText"`
	VBnoInH      flexInt  `json:"vBnoInH"`
	VBnoInX      flexInt  `json:"vBnoInX"`
	VBnoInY      flexInt  `json:"vBnoInY"`
	SDILevelA    flexBool `json:"sdilevelA"`
	Use1080p     flexBool `json:"use1080pNotPsf"`
	VSDIBuffer   flexBool `json:"vSDIBuffer"`
	VResize      flexBool `json:"vResize"`
	Mode         flexInt  `json:"mode"`
	Address      string   `json:"address"`
	Port         flexInt  `json:"port"`
	LocalPort    flexInt  `json:"localPort"`
	LocalPortOn  flexBool `json:"localPortOn"`
	Latency      flexInt  `json:"latency"`
	Encryption   flexBool `json:"encrytion"`
	PBKeyLen     flexInt  `json:"pbkeylen"`
	Passphrase   string   `json:"passphrase"`

	wireStreamValues
	V2110RTPPayload flexInt  `json:"v2110RTPpayload"`
	A2110RTPPayload flexInt  `json:"a2110RTPpayload"`
	V2110SourceIP   string   `json:"v2110SDPSourceIp"`
	A2110SourceIP   string   `json:"a2110SDPSourceIp"`
	A2110SDPaCh     flexInt  `json:"a2110SDPaCh"`
	A2110PacketTime flexInt  `json:"a2110PacketTime"`
	V2110StopNoIn   flexBool `json:"v2110StopNoIn"`
	A2110StopNoIn   flexBool `json:"a2110StopNoIn"`
}

// uplink maps the device's "auto" to "".
func uplink(s string) string {
	if s == "auto" {
		return ""
	}
	return s
}

func (v *wireConfigValues) encoder(c *collector) EncoderConfig {
	video, audio := v.streams(c)
	return EncoderConfig{
		Name:                v.Name,
		AutoStart:           c.bool("autoStart", &v.AutoStart),
		BarsStandard:        VideoMode(v.VModeB),
		NoSignal:            NoSignalMode(c.int("vNoS", &v.VNoS)),
		Codec:               VideoCodec(c.int("vCodec", &v.VCodec)),
		BitDepth:            c.int("vBit", &v.VBit),
		Chroma:              c.int("vColor", &v.VColor),
		IntraRefresh:        c.bool("vPIntraOn", &v.VPIntraOn),
		Interlaced:          c.bool("vIENC", &v.VIENC),
		CRFOn:               c.bool("vCRFOn", &v.VCRFOn),
		CRF:                 c.int("vCRF", &v.VCRF),
		GOPOn:               c.bool("vGOPOn", &v.VGOPOn),
		GOP:                 c.int("vGOP", &v.VGOP),
		FECLevel:            c.int("vFEC", &v.VFEC),
		Bitrate:             c.int("vTBR", &v.VTBR),
		AudioMode:           AudioMode(c.int("aMode", &v.AMode)),
		AudioBitDepth:       c.int("aBit", &v.ABit),
		AudioChannels:       c.int("aCh", &v.ACh),
		AudioFEC:            c.bool("aFEC", &v.AFEC),
		AudioBitrate:        c.int("aTBR", &v.ATBR),
		Card:                VideoCard(c.int("vCard", &v.VCard)),
		VideoMode:           VideoMode(v.VMode),
		VideoLock:           VideoMode(v.VModeLock),
		Video:               c.bool("video", &v.Video),
		Audio:               c.bool("audio", &v.Audio),
		Video2110:           video,
		Audio2110:           audio,
		Video2110Payload:    c.int("v2110RTPpayload", &v.V2110RTPPayload),
		Audio2110Payload:    c.int("a2110RTPpayload", &v.A2110RTPPayload),
		Video2110Source:     v.V2110SourceIP,
		Audio2110Source:     v.A2110SourceIP,
		Audio2110Channels:   c.int("a2110SDPaCh", &v.A2110SDPaCh),
		Audio2110PacketTime: c.int("a2110PacketTime", &v.A2110PacketTime),
		PacketAck:           c.bool("xAck", &v.XAck),
		Resend:              c.bool("xAckR", &v.XAckR),
		MaxResends:          c.int("xAckRmax", &v.XAckRmax),
		ResendDelay:         time.Duration(c.int("diffRtt", &v.DiffRtt)) * time.Millisecond,
		MaxRTT:              time.Duration(c.int("maxRtt", &v.MaxRtt)) * time.Millisecond,
		Uplink:              uplink(v.EthSoMark),
	}
}

func (v *wireConfigValues) decoder(c *collector) DecoderConfig {
	video, audio := v.streams(c)
	return DecoderConfig{
		Name:                    v.Name,
		AutoStart:               c.bool("autoStart", &v.AutoStart),
		Sender:                  unitID(v.Sender),
		AVSync:                  c.bool("vAVSOn", &v.VAVSOn),
		FPSSync:                 c.bool("vFRCOn", &v.VFRCOn),
		PacketBuffer:            time.Duration(c.int("pbuf", &v.PBuf)) * time.Millisecond,
		AllowLateFrames:         c.bool("late", &v.Late),
		SignalGen:               c.bool("vBnoInOn", &v.VBnoInOn),
		SignalGenDelay:          time.Duration(c.int("vBnoIn", &v.VBnoIn)) * time.Second,
		SignalGenType:           SignalGenerator(v.VBnoInType),
		OverlayTextOn:           c.bool("vBnoInTextOn", &v.VBnoInTextOn),
		OverlayText:             v.VBnoInText,
		OverlaySize:             c.int("vBnoInH", &v.VBnoInH),
		OverlayX:                c.int("vBnoInX", &v.VBnoInX),
		OverlayY:                c.int("vBnoInY", &v.VBnoInY),
		Card:                    VideoCard(c.int("vCard", &v.VCard)),
		VideoMode:               VideoMode(v.VMode),
		SDILevelA:               c.bool("sdilevelA", &v.SDILevelA),
		ProgressiveNotPsF:       c.bool("use1080pNotPsf", &v.Use1080p),
		Video2110:               video,
		Audio2110:               audio,
		Video2110Payload:        c.int("v2110RTPpayload", &v.V2110RTPPayload),
		Audio2110Payload:        c.int("a2110RTPpayload", &v.A2110RTPPayload),
		Video2110StopOnNoSignal: c.bool("v2110StopNoIn", &v.V2110StopNoIn),
		Audio2110StopOnNoSignal: c.bool("a2110StopNoIn", &v.A2110StopNoIn),
		Audio2110PacketTime:     c.int("a2110PacketTime", &v.A2110PacketTime),
		Uplink:                  uplink(v.EthSoMark),
	}
}

func (v *wireConfigValues) srtEncoder(c *collector) SRTEncoderConfig {
	return SRTEncoderConfig{
		Name:            v.Name,
		AutoStart:       c.bool("autoStart", &v.AutoStart),
		Mode:            SRTMode(c.int("mode", &v.Mode)),
		Address:         v.Address,
		Port:            c.int("port", &v.Port),
		LocalPort:       c.int("localPort", &v.LocalPort),
		FixedSourcePort: c.bool("localPortOn", &v.LocalPortOn),
		Latency:         time.Duration(c.int("latency", &v.Latency)) * time.Millisecond,
		Encryption:      c.bool("encrytion", &v.Encryption),
		KeyLength:       SRTKeyLength(c.int("pbkeylen", &v.PBKeyLen)),
		Passphrase:      v.Passphrase,
		BitDepth:        c.int("vBit", &v.VBit),
		IntraRefresh:    c.bool("vPIntraOn", &v.VPIntraOn),
		GOPOn:           c.bool("vGOPOn", &v.VGOPOn),
		GOP:             c.int("vGOP", &v.VGOP),
		Bitrate:         c.int("vTBR", &v.VTBR),
		AudioBitrate:    c.int("aTBR", &v.ATBR),
		Card:            VideoCard(c.int("vCard", &v.VCard)),
		VideoMode:       VideoMode(v.VMode),
		AudioChannels:   c.int("aCh", &v.ACh),
	}
}

func (v *wireConfigValues) srtDecoder(c *collector) SRTDecoderConfig {
	return SRTDecoderConfig{
		Name:           v.Name,
		AutoStart:      c.bool("autoStart", &v.AutoStart),
		Mode:           SRTMode(c.int("mode", &v.Mode)),
		Address:        v.Address,
		Port:           c.int("port", &v.Port),
		LocalPort:      c.int("localPort", &v.LocalPort),
		FixedLocalPort: c.bool("localPortOn", &v.LocalPortOn),
		Latency:        time.Duration(c.int("latency", &v.Latency)) * time.Millisecond,
		Encryption:     c.bool("encrytion", &v.Encryption),
		KeyLength:      SRTKeyLength(c.int("pbkeylen", &v.PBKeyLen)),
		Passphrase:     v.Passphrase,
		Card:           VideoCard(c.int("vCard", &v.VCard)),
		Audio:          c.bool("audio", &v.Audio),
		AudioCodec:     SRTAudioCodec(c.int("aMode", &v.AMode)),
	}
}

func (v *wireConfigValues) ndiEncoder(c *collector) NDIEncoderConfig {
	return NDIEncoderConfig{
		Name:          v.Name,
		AutoStart:     c.bool("autoStart", &v.AutoStart),
		Card:          VideoCard(c.int("vCard", &v.VCard)),
		VideoMode:     VideoMode(v.VMode),
		BarsStandard:  VideoMode(v.VModeB),
		AudioChannels: c.int("aCh", &v.ACh),
	}
}

func (v *wireConfigValues) ndiDecoder(c *collector) NDIDecoderConfig {
	return NDIDecoderConfig{
		Name:              v.Name,
		AutoStart:         c.bool("autoStart", &v.AutoStart),
		AVSync:            c.bool("vAVSOn", &v.VAVSOn),
		FPSSync:           c.bool("vFRCOn", &v.VFRCOn),
		SDIBuffer:         c.bool("vSDIBuffer", &v.VSDIBuffer),
		AudioChannels:     c.int("aCh", &v.ACh),
		Card:              VideoCard(c.int("vCard", &v.VCard)),
		VideoMode:         VideoMode(v.VMode),
		SDILevelA:         c.bool("sdilevelA", &v.SDILevelA),
		ProgressiveNotPsF: c.bool("use1080pNotPsf", &v.Use1080p),
		Resize:            c.bool("vResize", &v.VResize),
	}
}

// maxPendingUpdates limits the state.update messages kept for a unit whose
// state.subscribe response has not been applied yet.
const maxPendingUpdates = 16

// unitConfig is the configuration of one subscribed unit, merged from the
// state.subscribe response and the following state.update deltas.
type unitConfig struct {
	values wireConfigValues
	dataID int
	loaded bool
	// pending are updates received before the response, applied after it.
	pending []json.RawMessage
}

// wireConfigMessage is the response to state.subscribe and the params of
// state.update.
type wireConfigMessage struct {
	ID     string  `json:"id"`
	DataID flexInt `json:"dataid"`
	Data   struct {
		Values json.RawMessage `json:"values"`
	} `json:"data"`
}

// configs are the configurations of the subscribed units.
type configs map[UnitID]*unitConfig

// applySubscribe replaces the configuration of a unit with a state.subscribe
// response and applies the updates received before it.
func (cs configs) applySubscribe(raw json.RawMessage) (UnitID, error) {
	var m wireConfigMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", fmt.Errorf("xlinkclient: decode state.subscribe response: %w", err)
	}
	id := UnitID(m.ID)
	uc := &unitConfig{dataID: m.DataID.Value, loaded: true}
	if err := decodeConfigValues(m.Data.Values, &uc.values); err != nil {
		return id, err
	}
	var pending []json.RawMessage
	if old := cs[id]; old != nil {
		pending = old.pending
	}
	cs[id] = uc
	for _, p := range pending {
		if _, err := cs.applyUpdate(p); err != nil && !errors.Is(err, errStaleUpdate) {
			return id, err
		}
	}
	return id, nil
}

// applyUpdate merges a state.update message. An update for a unit whose
// response is still outstanding is kept until the response arrives; one that
// is not newer than the configuration is reported as errStaleUpdate. gap
// reports whether updates were lost, judged by dataid.
func (cs configs) applyUpdate(raw json.RawMessage) (gap bool, err error) {
	var m wireConfigMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false, fmt.Errorf("xlinkclient: decode state.update: %w", err)
	}
	id := UnitID(m.ID)
	uc := cs[id]
	if uc == nil {
		return false, errUnknownUnit
	}
	if !uc.loaded {
		if len(uc.pending) < maxPendingUpdates {
			uc.pending = append(uc.pending, raw)
		}
		return false, nil
	}
	if m.DataID.Value <= uc.dataID {
		return false, errStaleUpdate
	}
	if len(m.Data.Values) > 0 {
		if err := decodeConfigValues(m.Data.Values, &uc.values); err != nil {
			return false, err
		}
	}
	gap = m.DataID.Value != uc.dataID+1
	uc.dataID = m.DataID.Value
	return gap, nil
}

// errUnknownUnit is returned for a state.update of a unit that is not
// subscribed.
var errUnknownUnit = errors.New("xlinkclient: update for a unit that is not subscribed")

// expect prepares the configuration of a unit that is being subscribed, so
// that updates arriving before the response are kept.
func (cs configs) expect(id UnitID) {
	if cs[id] == nil {
		cs[id] = &unitConfig{}
	}
}

// decodeConfigValues decodes values into v. Type mismatches of single fields
// are tolerated as in decodeParams.
func decodeConfigValues(raw json.RawMessage, v *wireConfigValues) error {
	if len(raw) == 0 {
		return nil
	}
	err := json.Unmarshal(raw, v)
	var typeErr *json.UnmarshalTypeError
	if err != nil && !errors.As(err, &typeErr) {
		return fmt.Errorf("xlinkclient: decode unit configuration: %w", err)
	}
	return nil
}

// get returns the loaded configuration values of a unit.
func (cs configs) get(id UnitID) (*wireConfigValues, bool) {
	uc := cs[id]
	if uc == nil || !uc.loaded {
		return nil, false
	}
	return &uc.values, true
}

// attach adds the loaded configurations to the units of sys.
func (cs configs) attach(c *collector, sys *System) {
	cc := c.scope("config")
	for i := range sys.Encoders {
		e := &sys.Encoders[i]
		if v, ok := cs.get(e.ID); ok {
			cfg := v.encoder(cc.scope(string(e.ID)))
			e.Config = &cfg
		}
		if v, ok := cs.get(e.Receiver.ID); ok && e.Receiver.ID != "" {
			cfg := v.decoder(cc.scope(string(e.Receiver.ID)))
			e.Receiver.Config = &cfg
		}
	}
	for i := range sys.Decoders {
		d := &sys.Decoders[i]
		if v, ok := cs.get(d.ID); ok {
			cfg := v.decoder(cc.scope(string(d.ID)))
			d.Config = &cfg
		}
		if v, ok := cs.get(d.Sender.ID); ok && d.Sender.ID != "" {
			cfg := v.encoder(cc.scope(string(d.Sender.ID)))
			d.Sender.Config = &cfg
		}
	}
	for i := range sys.SRTEncoders {
		if v, ok := cs.get(sys.SRTEncoders[i].ID); ok {
			cfg := v.srtEncoder(cc.scope(string(sys.SRTEncoders[i].ID)))
			sys.SRTEncoders[i].Config = &cfg
		}
	}
	for i := range sys.SRTDecoders {
		if v, ok := cs.get(sys.SRTDecoders[i].ID); ok {
			cfg := v.srtDecoder(cc.scope(string(sys.SRTDecoders[i].ID)))
			sys.SRTDecoders[i].Config = &cfg
		}
	}
	for i := range sys.NDIEncoders {
		if v, ok := cs.get(sys.NDIEncoders[i].ID); ok {
			cfg := v.ndiEncoder(cc.scope(string(sys.NDIEncoders[i].ID)))
			sys.NDIEncoders[i].Config = &cfg
		}
	}
	for i := range sys.NDIDecoders {
		if v, ok := cs.get(sys.NDIDecoders[i].ID); ok {
			cfg := v.ndiDecoder(cc.scope(string(sys.NDIDecoders[i].ID)))
			sys.NDIDecoders[i].Config = &cfg
		}
	}
	c.merge(cc)
}

// subscribable returns the units whose configuration is subscribed: all local
// units and the counterparts of the local XLink units.
func (s System) subscribable() map[UnitID]bool {
	ids := map[UnitID]bool{}
	for _, e := range s.Encoders {
		ids[e.ID] = true
		if e.Receiver.ID != "" {
			ids[e.Receiver.ID] = true
		}
	}
	for _, d := range s.Decoders {
		ids[d.ID] = true
		if d.Sender.ID != "" {
			ids[d.Sender.ID] = true
		}
	}
	for _, u := range s.SRTEncoders {
		ids[u.ID] = true
	}
	for _, u := range s.SRTDecoders {
		ids[u.ID] = true
	}
	for _, u := range s.NDIEncoders {
		ids[u.ID] = true
	}
	for _, u := range s.NDIDecoders {
		ids[u.ID] = true
	}
	return ids
}
