package xlinkclient

import "time"

// Signal describes an input or output as reported by the device: either
// NoSignal or a description of the detected format.
type Signal string

// NoSignal is reported for inputs and outputs without a signal.
const NoSignal Signal = "No Signal"

// Present reports whether a signal is present.
func (s Signal) Present() bool {
	return s != "" && s != NoSignal
}

// Stream is the primary SMPTE ST 2110 stream of a unit.
type Stream struct {
	// Interface is the network interface, e.g. "eth6", or "" if none is set.
	Interface string
	Enabled   bool
	// Address is the multicast address.
	Address string
	Port    int
}

// Encoder is an XLink encoder of the local system.
type Encoder struct {
	ID      UnitID
	Name    string
	Enabled bool
	Running bool
	Card    VideoCard
	// Connector is the connector type of Card, e.g. "sdi".
	Connector string
	VideoIn   Signal
	AudioIn   Signal
	// HDR is the dynamic range of the video input.
	HDR HDRMode
	// NDISource is the selected NDI source if Card is VideoCardNDI, and
	// NDISourceName the name of the NDI host providing it.
	NDISource          string
	NDISourceName      string
	NDISourceConnected bool
	// MultiPointSource is the selected MultiPoint sender if Card is
	// VideoCardMultiPoint, or "" if none is selected.
	MultiPointSource string
	Codec            VideoCodec
	// CRF reports whether constant rate factor encoding is on; Bitrate is
	// then the maximum bitrate.
	CRF bool
	// Bitrate is the target bitrate in Mbps.
	Bitrate   int
	FECLevel  int
	Video2110 Stream
	Audio2110 Stream
	// XLink reports whether the XLink tunnel to the receiver is up.
	XLink bool
	// P2P reports whether the tunnel is a direct peer-to-peer connection.
	P2P bool
	// Receiver is the decoder this encoder sends to. Its ID is empty if no
	// receiver is selected.
	Receiver LinkedReceiver
	// Config is the configuration of the unit, or nil until it has been
	// loaded.
	Config *EncoderConfig
}

// Decoder is an XLink decoder of the local system.
type Decoder struct {
	ID      UnitID
	Name    string
	Enabled bool
	Running bool
	Card    VideoCard
	// Connector is the connector type of Card, e.g. "sdi".
	Connector string
	VideoIn   Signal
	AudioIn   Signal
	VideoOut  Signal
	AudioOut  Signal
	// HDR is the dynamic range of the video output.
	HDR HDRMode
	// RefLocked reports whether the output is locked to the reference.
	RefLocked bool
	// AVSync reports whether audio and video are in sync; otherwise AVDelay
	// is the offset between them.
	AVSync  bool
	AVDelay time.Duration
	// SignalGenOn enables the signal generator when no input is received.
	SignalGenOn bool
	// SignalGenDelay is the time without input before the generator starts.
	SignalGenDelay time.Duration
	FPSSync        bool
	Video2110      Stream
	Audio2110      Stream
	// XLink reports whether the XLink tunnel to the sender is up.
	XLink bool
	// P2P reports whether the tunnel is a direct peer-to-peer connection.
	P2P bool
	// Sender is the encoder this decoder receives from. Its ID is empty if no
	// sender is selected.
	Sender LinkedSender
	// Config is the configuration of the unit, or nil until it has been
	// loaded.
	Config *DecoderConfig
}

// SRTEncoder is an SRT sender of the local system.
type SRTEncoder struct {
	ID      UnitID
	Name    string
	Enabled bool
	Running bool
	// Error is the last error reported while running, or "".
	Error string
	Card  VideoCard
	// Connector is the connector type of Card, e.g. "sdi".
	Connector string
	VideoIn   Signal
	// Bitrate is the video bitrate in Mbps.
	Bitrate int
	SRT     SRTConnection
	// Config is the configuration of the unit, or nil until it has been
	// loaded.
	Config *SRTEncoderConfig
}

// SRTDecoder is an SRT receiver of the local system.
type SRTDecoder struct {
	ID      UnitID
	Name    string
	Enabled bool
	Running bool
	// Error is the last error reported while running, or "".
	Error string
	Card  VideoCard
	// Connector is the connector type of Card, e.g. "sdi".
	Connector string
	VideoIn   Signal
	VideoOut  Signal
	SRT       SRTConnection
	// Config is the configuration of the unit, or nil until it has been
	// loaded.
	Config *SRTDecoderConfig
}

// SRTConnection describes how an SRT unit connects.
type SRTConnection struct {
	Mode SRTMode
	// Address and Port are the remote endpoint in caller mode.
	Address string
	Port    int
	// LocalPort is the port to listen on in listener mode, or the source port
	// in caller mode.
	LocalPort  int
	Encryption bool
}

// NDIEncoder converts an SDI input of the local system to an NDI stream.
type NDIEncoder struct {
	ID      UnitID
	Name    string
	Enabled bool
	Running bool
	Card    VideoCard
	// Connector is the connector type of Card, e.g. "sdi".
	Connector string
	VideoIn   Signal
	VideoOut  Signal
	// Config is the configuration of the unit, or nil until it has been
	// loaded.
	Config *NDIEncoderConfig
}

// NDIDecoder outputs an NDI stream on an SDI output of the local system.
type NDIDecoder struct {
	ID      UnitID
	Name    string
	Enabled bool
	Running bool
	Card    VideoCard
	// Connector is the connector type of Card, e.g. "sdi".
	Connector string
	VideoIn   Signal
	VideoOut  Signal
	// RefLocked reports whether the output is locked to the reference.
	RefLocked bool
	// AVSync reports whether audio and video are in sync; otherwise AVDelay
	// is the offset between them.
	AVSync  bool
	AVDelay time.Duration
	// Source is the selected NDI source and SourceName its display name.
	Source          string
	SourceName      string
	SourceConnected bool
	FPSSync         bool
	// Config is the configuration of the unit, or nil until it has been
	// loaded.
	Config *NDIDecoderConfig
}

// LinkedUnit is the counterpart of a local unit as reported by the local
// system: the receiver of an encoder or the sender of a decoder.
type LinkedUnit struct {
	ID         UnitID
	Name       string
	SystemName string
	Running    bool
	Connected  bool
	Card       VideoCard
	// Connector is the connector type of Card, e.g. "sdi".
	Connector string
	VideoIn   Signal
	AudioIn   Signal
	// HDR is the dynamic range of the video input of a sender or of the video
	// output of a receiver.
	HDR HDRMode
}

// LinkedReceiver is the decoder a local encoder sends to.
type LinkedReceiver struct {
	LinkedUnit
	VideoOut  Signal
	AudioOut  Signal
	RefLocked bool
	// AVSync reports whether audio and video are in sync; otherwise AVDelay
	// is the offset between them.
	AVSync  bool
	AVDelay time.Duration
	// Config is the configuration of the receiver, or nil until it has been
	// loaded.
	Config *DecoderConfig
}

// LinkedSender is the encoder a local decoder receives from.
type LinkedSender struct {
	LinkedUnit
	// NDISource is the selected NDI source if Card is VideoCardNDI.
	NDISource          string
	NDISourceConnected bool
	// MultiPointSource is the selected MultiPoint sender if Card is
	// VideoCardMultiPoint, or "" if none is selected.
	MultiPointSource string
	Codec            VideoCodec
	// CRF reports whether constant rate factor encoding is on; Bitrate is
	// then the maximum bitrate.
	CRF bool
	// Bitrate is the target bitrate in Mbps.
	Bitrate int
	// Config is the configuration of the sender, or nil until it has been
	// loaded.
	Config *EncoderConfig
}
