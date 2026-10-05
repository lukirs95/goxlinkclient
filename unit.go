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
	ID        UnitID
	Name      string
	Enabled   bool
	Running   bool
	StartedAt time.Time
	Card      VideoCard
	VideoIn   Signal
	AudioIn   Signal
	Codec     VideoCodec
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
	Receiver LinkedUnit
}

// Decoder is an XLink decoder of the local system.
type Decoder struct {
	ID        UnitID
	Name      string
	Enabled   bool
	Running   bool
	StartedAt time.Time
	Card      VideoCard
	VideoIn   Signal
	AudioIn   Signal
	VideoOut  Signal
	AudioOut  Signal
	// AudioChannels is the number of received audio channels.
	AudioChannels int
	// SignalGenOn enables the signal generator when no input is received.
	SignalGenOn bool
	// SignalGenDelay is the time without input before the generator starts.
	SignalGenDelay time.Duration
	BufferOn       bool
	Buffer         int
	FPSSync        bool
	Video2110      Stream
	Audio2110      Stream
	// XLink reports whether the XLink tunnel to the sender is up.
	XLink bool
	// P2P reports whether the tunnel is a direct peer-to-peer connection.
	P2P bool
	// Sender is the encoder this decoder receives from. Its ID is empty if no
	// sender is selected.
	Sender LinkedUnit
}

// LinkedUnit is the counterpart of a local unit as reported by the local
// system: the receiver of an encoder or the sender of a decoder.
type LinkedUnit struct {
	ID         UnitID
	Name       string
	SystemName string
	Running    bool
	Connected  bool
	P2P        bool
	OnLocalNet bool
	// Video and Audio are the signals of the counterpart: its inputs for a
	// sender, its outputs for a receiver.
	Video Signal
	Audio Signal
}
