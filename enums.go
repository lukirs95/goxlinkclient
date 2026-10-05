package xlinkclient

import "strconv"

// The constants in this file mirror the option lists the device sends with
// state.subscribe. Their values are the IDs used on the wire.

// VideoCard selects the physical or virtual video interface of a unit
// (key "vCard"). On the wire it is a string holding the number.
type VideoCard int

const (
	VideoCardNone   VideoCard = 0
	VideoCardSDI1   VideoCard = 1
	VideoCardSDI2   VideoCard = 2
	VideoCardSDI3   VideoCard = 3
	VideoCardSDI4   VideoCard = 4
	VideoCardSDI5   VideoCard = 5
	VideoCardSDI6   VideoCard = 6
	VideoCardSDI7   VideoCard = 7
	VideoCardSDI8   VideoCard = 8
	VideoCardNDI    VideoCard = 10
	VideoCardST2110 VideoCard = 12
)

// SDIPort returns the SDI port number (1-8). ok is false if the card is not an
// SDI interface.
func (c VideoCard) SDIPort() (port int, ok bool) {
	if c >= VideoCardSDI1 && c <= VideoCardSDI8 {
		return int(c), true
	}
	return 0, false
}

func (c VideoCard) String() string {
	switch {
	case c == VideoCardNone:
		return "None"
	case c == VideoCardNDI:
		return "NDI"
	case c == VideoCardST2110:
		return "2110"
	case c >= VideoCardSDI1 && c <= VideoCardSDI8:
		return "SDI (" + strconv.Itoa(int(c)) + ")"
	default:
		return "VideoCard(" + strconv.Itoa(int(c)) + ")"
	}
}

// VideoMode is a video standard (keys "vMode", "vModeB", "vModeLock").
type VideoMode string

const (
	VideoModeAuto      VideoMode = "auto"
	VideoModeNTSC      VideoMode = "ntsc"
	VideoModePAL       VideoMode = "pal"
	VideoMode720p50    VideoMode = "hp50"
	VideoMode720p5994  VideoMode = "hp59"
	VideoMode720p60    VideoMode = "hp60"
	VideoMode1080p2398 VideoMode = "23ps"
	VideoMode1080p24   VideoMode = "24ps"
	VideoMode1080p25   VideoMode = "Hp25"
	VideoMode1080p2997 VideoMode = "Hp29"
	VideoMode1080p30   VideoMode = "Hp30"
	VideoMode1080i50   VideoMode = "Hi50"
	VideoMode1080i5994 VideoMode = "Hi59"
	VideoMode1080i60   VideoMode = "Hi60"
	VideoMode1080p50   VideoMode = "Hp50"
	VideoMode1080p5994 VideoMode = "Hp59"
	VideoMode1080p60   VideoMode = "Hp60"
)

// VideoCodec is the codec of an XLink encoder (key "vCodec").
type VideoCodec int

const (
	VideoCodecH264 VideoCodec = 1
	VideoCodecH265 VideoCodec = 2
	VideoCodecVC5  VideoCodec = 4
)

func (c VideoCodec) String() string {
	switch c {
	case VideoCodecH264:
		return "H.264"
	case VideoCodecH265:
		return "H.265"
	case VideoCodecVC5:
		return "VC-5"
	default:
		return "VideoCodec(" + strconv.Itoa(int(c)) + ")"
	}
}

// NoSignalMode is what an XLink encoder sends without input (key "vNoS").
type NoSignalMode int

const (
	NoSignalOff       NoSignalMode = 0
	NoSignalBars      NoSignalMode = 1
	NoSignalLastFrame NoSignalMode = 2
)

// AudioMode is the audio compression of an XLink encoder (key "aMode").
type AudioMode int

const (
	AudioUncompressed AudioMode = 1
	AudioCompressed   AudioMode = 2
)

// SignalGenerator is what a decoder outputs while it has no input (key
// "vBnoInType").
type SignalGenerator string

const (
	SignalGeneratorBars  SignalGenerator = "Bars"
	SignalGeneratorBlack SignalGenerator = "Black"
)

// VideoConversion is the output conversion of a decoder (key "vModeC").
type VideoConversion string

const (
	VideoConversionNone       VideoConversion = "none"
	VideoConversionLetterbox  VideoConversion = "ltbx"
	VideoConversionAnamorphic VideoConversion = "amph"
	VideoConversion720To1080  VideoConversion = "720c"
)

// SRTMode is the connection mode of an SRT unit (key "mode").
type SRTMode int

const (
	SRTListener SRTMode = 1
	SRTCaller   SRTMode = 2
)

func (m SRTMode) String() string {
	switch m {
	case SRTListener:
		return "Listener"
	case SRTCaller:
		return "Caller"
	default:
		return "SRTMode(" + strconv.Itoa(int(m)) + ")"
	}
}

// SRTKeyLength is the AES key length of an encrypted SRT stream in bytes (key
// "pbkeylen").
type SRTKeyLength int

const (
	SRTAES128 SRTKeyLength = 16
	SRTAES192 SRTKeyLength = 24
	SRTAES256 SRTKeyLength = 32
)

// SRTAudioCodec is the audio codec an SRT decoder expects (key "aMode").
type SRTAudioCodec int

const (
	SRTAudioAuto   SRTAudioCodec = 0
	SRTAudioAAC    SRTAudioCodec = 1
	SRTAudioMP2    SRTAudioCodec = 2
	SRTAudioOpus   SRTAudioCodec = 3
	SRTAudioAAC2Ch SRTAudioCodec = 4
)
