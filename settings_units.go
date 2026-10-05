package xlinkclient

import (
	"context"
	"strconv"
	"time"
)

// Settings are typed values for the config request. Each constructor encodes
// its value exactly as the web UI does: numeric text fields such as ports and
// RTP payload IDs are sent as strings, an empty UnitID or interface as "none".
// Every unit kind has its own setting type, so a decoder setting cannot be
// passed to an encoder by mistake.

// EncoderSetting changes a setting of an XLink encoder. See ConfigureEncoder.
type EncoderSetting func(*fields)

// DecoderSetting changes a setting of an XLink decoder. See ConfigureDecoder.
type DecoderSetting func(*fields)

// SRTEncoderSetting changes a setting of an SRT encoder. See
// ConfigureSRTEncoder.
type SRTEncoderSetting func(*fields)

// SRTDecoderSetting changes a setting of an SRT decoder. See
// ConfigureSRTDecoder.
type SRTDecoderSetting func(*fields)

// NDIEncoderSetting changes a setting of an NDI encoder. See
// ConfigureNDIEncoder.
type NDIEncoderSetting func(*fields)

// NDIDecoderSetting changes a setting of an NDI decoder. See
// ConfigureNDIDecoder.
type NDIDecoderSetting func(*fields)

// ConfigureEncoder changes settings of an XLink encoder, which may belong to a
// remote system. All settings are sent in one request.
func (c *Client) ConfigureEncoder(ctx context.Context, id UnitID, settings ...EncoderSetting) error {
	return c.configure(ctx, id, collect(settings))
}

// ConfigureDecoder changes settings of an XLink decoder, which may belong to a
// remote system. All settings are sent in one request.
func (c *Client) ConfigureDecoder(ctx context.Context, id UnitID, settings ...DecoderSetting) error {
	return c.configure(ctx, id, collect(settings))
}

// ConfigureSRTEncoder changes settings of an SRT encoder. All settings are
// sent in one request.
func (c *Client) ConfigureSRTEncoder(ctx context.Context, id UnitID, settings ...SRTEncoderSetting) error {
	return c.configure(ctx, id, collect(settings))
}

// ConfigureSRTDecoder changes settings of an SRT decoder. All settings are
// sent in one request.
func (c *Client) ConfigureSRTDecoder(ctx context.Context, id UnitID, settings ...SRTDecoderSetting) error {
	return c.configure(ctx, id, collect(settings))
}

// ConfigureNDIEncoder changes settings of an NDI encoder. All settings are
// sent in one request.
func (c *Client) ConfigureNDIEncoder(ctx context.Context, id UnitID, settings ...NDIEncoderSetting) error {
	return c.configure(ctx, id, collect(settings))
}

// ConfigureNDIDecoder changes settings of an NDI decoder. All settings are
// sent in one request.
func (c *Client) ConfigureNDIDecoder(ctx context.Context, id UnitID, settings ...NDIDecoderSetting) error {
	return c.configure(ctx, id, collect(settings))
}

func (c *Client) configure(ctx context.Context, id UnitID, f fields) error {
	if len(f) == 0 {
		return nil
	}
	return c.callUnit(ctx, methodConfig, c.SystemID(), string(id), f.object())
}

// collect applies settings of any kind to a new set of fields.
func collect[S ~func(*fields)](settings []S) fields {
	var f fields
	for _, s := range settings {
		s(&f)
	}
	return f
}

// set returns a setting of kind S that sets key to value.
func set[S ~func(*fields)](key string, value any) S {
	return S(func(f *fields) { f.set(key, value) })
}

// Wire encodings used by the web UI.

func numText(n int) string { return strconv.Itoa(n) }

func boolText(b bool) string { return strconv.FormatBool(b) }

func millis(d time.Duration) int { return int(d / time.Millisecond) }

func unitRef(id UnitID) string {
	if id == "" {
		return "none"
	}
	return string(id)
}

func ifaceRef(name string) string {
	if name == "" {
		return "none"
	}
	return name
}

func uplinkRef(name string) string {
	if name == "" {
		return "auto"
	}
	return name
}

// stream sets the primary SMPTE ST 2110 keys for prefix "v" (video) or "a"
// (audio).
func stream[S ~func(*fields)](prefix string, s Stream) S {
	return S(func(f *fields) {
		f.set(prefix+"2110NetPri", ifaceRef(s.Interface))
		f.set(prefix+"2110NetPriEnabled", s.Enabled)
		f.set(prefix+"2110NetPriIp", s.Address)
		f.set(prefix+"2110NetPriPort", numText(s.Port))
	})
}

// XLink encoder settings.

// EncoderName renames the encoder.
func EncoderName(name string) EncoderSetting { return set[EncoderSetting]("name", name) }

// EncoderAutoStart starts the encoder automatically after boot.
func EncoderAutoStart(on bool) EncoderSetting { return set[EncoderSetting]("autoStart", on) }

// EncoderBarsStandard sets the video standard of the test pattern.
func EncoderBarsStandard(m VideoMode) EncoderSetting {
	return set[EncoderSetting]("vModeB", string(m))
}

// EncoderNoSignal sets what is sent while there is no input.
func EncoderNoSignal(m NoSignalMode) EncoderSetting { return set[EncoderSetting]("vNoS", int(m)) }

// EncoderCodec sets the video codec.
func EncoderCodec(c VideoCodec) EncoderSetting { return set[EncoderSetting]("vCodec", int(c)) }

// EncoderBitDepth sets the video bit depth (8 or 10).
func EncoderBitDepth(bits int) EncoderSetting { return set[EncoderSetting]("vBit", bits) }

// EncoderChroma sets the chroma subsampling (420 or 422).
func EncoderChroma(subsampling int) EncoderSetting {
	return set[EncoderSetting]("vColor", subsampling)
}

// EncoderIntraRefresh enables periodic intra refresh.
func EncoderIntraRefresh(on bool) EncoderSetting { return set[EncoderSetting]("vPIntraOn", on) }

// EncoderInterlaced enables interlaced encoding.
func EncoderInterlaced(on bool) EncoderSetting { return set[EncoderSetting]("vIENC", on) }

// EncoderCRF enables constant rate factor encoding with the given value. If
// on is false the encoder uses a constant bitrate.
func EncoderCRF(on bool, value int) EncoderSetting {
	return func(f *fields) {
		f.set("vCRFOn", on)
		f.set("vCRF", value)
	}
}

// EncoderGOP sets a fixed GOP size. If on is false the GOP size is automatic.
func EncoderGOP(on bool, size int) EncoderSetting {
	return func(f *fields) {
		f.set("vGOPOn", on)
		f.set("vGOP", size)
	}
}

// EncoderFEC sets the video forward error correction level (0-3).
func EncoderFEC(level int) EncoderSetting { return set[EncoderSetting]("vFEC", level) }

// EncoderBitrate sets the target video bitrate in Mbps.
func EncoderBitrate(mbps int) EncoderSetting { return set[EncoderSetting]("vTBR", mbps) }

// EncoderAudioMode sets the audio compression.
func EncoderAudioMode(m AudioMode) EncoderSetting { return set[EncoderSetting]("aMode", int(m)) }

// EncoderAudioBitDepth sets the audio bit depth (16 or 32).
func EncoderAudioBitDepth(bits int) EncoderSetting { return set[EncoderSetting]("aBit", bits) }

// EncoderAudioChannels sets the number of audio channels (2, 8 or 16).
func EncoderAudioChannels(n int) EncoderSetting { return set[EncoderSetting]("aCh", n) }

// EncoderAudioFEC enables audio forward error correction.
func EncoderAudioFEC(on bool) EncoderSetting { return set[EncoderSetting]("aFEC", on) }

// EncoderAudioBitrate sets the compressed audio bitrate per channel in kbps.
func EncoderAudioBitrate(kbps int) EncoderSetting { return set[EncoderSetting]("aTBR", kbps) }

// EncoderCard selects the video input.
func EncoderCard(card VideoCard) EncoderSetting {
	return set[EncoderSetting]("vCard", numText(int(card)))
}

// EncoderVideoMode sets the expected video standard of the input.
func EncoderVideoMode(m VideoMode) EncoderSetting { return set[EncoderSetting]("vMode", string(m)) }

// EncoderVideoLock locks the video standard of a 2110 input; VideoModeAuto
// disables the lock.
func EncoderVideoLock(m VideoMode) EncoderSetting {
	return set[EncoderSetting]("vModeLock", string(m))
}

// EncoderVideo enables the video essence of a 2110 input.
func EncoderVideo(on bool) EncoderSetting { return set[EncoderSetting]("video", on) }

// EncoderAudio enables audio of an SDI input.
func EncoderAudio(on bool) EncoderSetting { return set[EncoderSetting]("audio", on) }

// EncoderVideo2110 sets the primary 2110 video stream to receive.
func EncoderVideo2110(s Stream) EncoderSetting { return stream[EncoderSetting]("v", s) }

// EncoderAudio2110 sets the primary 2110 audio stream to receive.
func EncoderAudio2110(s Stream) EncoderSetting { return stream[EncoderSetting]("a", s) }

// EncoderVideo2110Payload sets the RTP payload ID of the 2110 video stream.
func EncoderVideo2110Payload(id int) EncoderSetting {
	return set[EncoderSetting]("v2110RTPpayload", numText(id))
}

// EncoderAudio2110Payload sets the RTP payload ID of the 2110 audio stream.
func EncoderAudio2110Payload(id int) EncoderSetting {
	return set[EncoderSetting]("a2110RTPpayload", numText(id))
}

// EncoderVideo2110Source sets the source IP of the 2110 video stream.
func EncoderVideo2110Source(ip string) EncoderSetting {
	return set[EncoderSetting]("v2110SDPSourceIp", ip)
}

// EncoderAudio2110Source sets the source IP of the 2110 audio stream.
func EncoderAudio2110Source(ip string) EncoderSetting {
	return set[EncoderSetting]("a2110SDPSourceIp", ip)
}

// EncoderAudio2110Channels sets the number of audio channels in the SDP.
func EncoderAudio2110Channels(n int) EncoderSetting {
	return set[EncoderSetting]("a2110SDPaCh", numText(n))
}

// EncoderAudio2110PacketTime sets the packet time of the 2110 audio stream,
// using the option ID listed by the device (2 is 0.125 ms).
func EncoderAudio2110PacketTime(id int) EncoderSetting {
	return set[EncoderSetting]("a2110PacketTime", id)
}

// EncoderPacketAck enables packet acknowledgement, required for resending.
func EncoderPacketAck(on bool) EncoderSetting { return set[EncoderSetting]("xAck", on) }

// EncoderResend enables resending of lost packets (ARQ).
func EncoderResend(on bool) EncoderSetting { return set[EncoderSetting]("xAckR", on) }

// EncoderMaxResends sets the number of retransmission attempts.
func EncoderMaxResends(n int) EncoderSetting { return set[EncoderSetting]("xAckRmax", n) }

// EncoderResendDelay sets the window for retransmissions.
func EncoderResendDelay(d time.Duration) EncoderSetting {
	return set[EncoderSetting]("diffRtt", millis(d))
}

// EncoderMaxRTT sets the maximum round trip time for retransmissions.
func EncoderMaxRTT(d time.Duration) EncoderSetting {
	return set[EncoderSetting]("maxRtt", millis(d))
}

// EncoderUplink forces the XLink traffic to the given interface. An empty
// name selects the default uplink automatically.
func EncoderUplink(iface string) EncoderSetting {
	return set[EncoderSetting]("ethSoMark", uplinkRef(iface))
}

// XLink decoder settings.

// DecoderName renames the decoder.
func DecoderName(name string) DecoderSetting { return set[DecoderSetting]("name", name) }

// DecoderAutoStart starts the decoder automatically after boot.
func DecoderAutoStart(on bool) DecoderSetting { return set[DecoderSetting]("autoStart", on) }

// DecoderSender selects the encoder to receive from. An empty ID clears it.
//
// Links between units are always made on the decoder: the device ignores the
// "receiver" key of an encoder. To link a local encoder to a decoder of a
// remote system, configure the remote decoder with the local encoder as its
// sender.
func DecoderSender(id UnitID) DecoderSetting { return set[DecoderSetting]("sender", unitRef(id)) }

// DecoderAVSync enables audio/video synchronisation.
func DecoderAVSync(on bool) DecoderSetting { return set[DecoderSetting]("vAVSOn", on) }

// DecoderFPSSync enables frame rate synchronisation.
func DecoderFPSSync(on bool) DecoderSetting { return set[DecoderSetting]("vFRCOn", on) }

// DecoderPacketBuffer sets the receive buffer. The device sets the maximum
// round trip time to the buffer plus 100 ms.
func DecoderPacketBuffer(d time.Duration) DecoderSetting {
	return set[DecoderSetting]("pbuf", millis(d))
}

// DecoderAllowLateFrames outputs late frames instead of dropping them; this
// can build up delay.
func DecoderAllowLateFrames(on bool) DecoderSetting { return set[DecoderSetting]("late", on) }

// DecoderSignalGen enables the signal generator while there is no input.
func DecoderSignalGen(on bool) DecoderSetting { return set[DecoderSetting]("vBnoInOn", on) }

// DecoderSignalGenDelay sets the time without input before the signal
// generator starts.
func DecoderSignalGenDelay(d time.Duration) DecoderSetting {
	return set[DecoderSetting]("vBnoIn", int(d/time.Second))
}

// DecoderSignalGenType sets what the signal generator outputs.
func DecoderSignalGenType(g SignalGenerator) DecoderSetting {
	return set[DecoderSetting]("vBnoInType", string(g))
}

// DecoderOverlayText enables the info overlay with the given template. The
// template may use %system%, %sysname%, %name% and %format%.
func DecoderOverlayText(on bool, template string) DecoderSetting {
	return func(f *fields) {
		f.set("vBnoInTextOn", on)
		f.set("vBnoInText", template)
	}
}

// DecoderOverlaySize sets the text size of the info overlay.
func DecoderOverlaySize(size int) DecoderSetting { return set[DecoderSetting]("vBnoInH", size) }

// DecoderOverlayPosition sets the position of the info overlay.
func DecoderOverlayPosition(x, y int) DecoderSetting {
	return func(f *fields) {
		f.set("vBnoInX", x)
		f.set("vBnoInY", y)
	}
}

// DecoderCard selects the video output.
func DecoderCard(card VideoCard) DecoderSetting {
	return set[DecoderSetting]("vCard", numText(int(card)))
}

// DecoderVideoMode sets the video standard of an SDI output.
func DecoderVideoMode(m VideoMode) DecoderSetting { return set[DecoderSetting]("vMode", string(m)) }

// DecoderSDILevelA selects SDI level A instead of B.
func DecoderSDILevelA(on bool) DecoderSetting {
	return set[DecoderSetting]("sdilevelA", boolText(on))
}

// DecoderProgressiveNotPsF outputs 1080p instead of PsF.
func DecoderProgressiveNotPsF(on bool) DecoderSetting {
	return set[DecoderSetting]("use1080pNotPsf", on)
}

// DecoderVideo2110 sets the primary 2110 video stream to send.
func DecoderVideo2110(s Stream) DecoderSetting { return stream[DecoderSetting]("v", s) }

// DecoderAudio2110 sets the primary 2110 audio stream to send.
func DecoderAudio2110(s Stream) DecoderSetting { return stream[DecoderSetting]("a", s) }

// DecoderVideo2110Payload sets the RTP payload ID of the 2110 video stream.
func DecoderVideo2110Payload(id int) DecoderSetting {
	return set[DecoderSetting]("v2110RTPpayload", numText(id))
}

// DecoderAudio2110Payload sets the RTP payload ID of the 2110 audio stream.
func DecoderAudio2110Payload(id int) DecoderSetting {
	return set[DecoderSetting]("a2110RTPpayload", numText(id))
}

// DecoderVideo2110StopOnNoSignal stops the 2110 video stream without input.
func DecoderVideo2110StopOnNoSignal(on bool) DecoderSetting {
	return set[DecoderSetting]("v2110StopNoIn", on)
}

// DecoderAudio2110StopOnNoSignal stops the 2110 audio stream without input.
func DecoderAudio2110StopOnNoSignal(on bool) DecoderSetting {
	return set[DecoderSetting]("a2110StopNoIn", on)
}

// DecoderAudio2110PacketTime sets the packet time of the 2110 audio stream,
// using the option ID listed by the device (2 is 0.125 ms).
func DecoderAudio2110PacketTime(id int) DecoderSetting {
	return set[DecoderSetting]("a2110PacketTime", id)
}

// DecoderUplink forces the XLink traffic to the given interface. An empty
// name selects the default uplink automatically.
func DecoderUplink(iface string) DecoderSetting {
	return set[DecoderSetting]("ethSoMark", uplinkRef(iface))
}

// SRT encoder settings.

// SRTEncoderName renames the SRT encoder.
func SRTEncoderName(name string) SRTEncoderSetting { return set[SRTEncoderSetting]("name", name) }

// SRTEncoderAutoStart starts the SRT encoder automatically after boot.
func SRTEncoderAutoStart(on bool) SRTEncoderSetting {
	return set[SRTEncoderSetting]("autoStart", on)
}

// SRTEncoderMode sets listener or caller mode.
func SRTEncoderMode(m SRTMode) SRTEncoderSetting { return set[SRTEncoderSetting]("mode", int(m)) }

// SRTEncoderAddress sets the remote host in caller mode.
func SRTEncoderAddress(host string) SRTEncoderSetting {
	return set[SRTEncoderSetting]("address", host)
}

// SRTEncoderPort sets the remote port in caller mode.
func SRTEncoderPort(port int) SRTEncoderSetting {
	return set[SRTEncoderSetting]("port", numText(port))
}

// SRTEncoderLocalPort sets the port to listen on in listener mode, or the
// source port in caller mode.
func SRTEncoderLocalPort(port int) SRTEncoderSetting {
	return set[SRTEncoderSetting]("localPort", numText(port))
}

// SRTEncoderFixedSourcePort uses the local port as source port in caller
// mode.
func SRTEncoderFixedSourcePort(on bool) SRTEncoderSetting {
	return set[SRTEncoderSetting]("localPortOn", on)
}

// SRTEncoderLatency sets the SRT latency.
func SRTEncoderLatency(d time.Duration) SRTEncoderSetting {
	return set[SRTEncoderSetting]("latency", numText(millis(d)))
}

// SRTEncoderEncryption enables AES encryption.
func SRTEncoderEncryption(on bool) SRTEncoderSetting {
	return set[SRTEncoderSetting]("encrytion", on)
}

// SRTEncoderKeyLength sets the AES key length.
func SRTEncoderKeyLength(k SRTKeyLength) SRTEncoderSetting {
	return set[SRTEncoderSetting]("pbkeylen", int(k))
}

// SRTEncoderPassphrase sets the encryption passphrase. Note that the device
// receives it in plain text over an unencrypted websocket.
func SRTEncoderPassphrase(p string) SRTEncoderSetting {
	return set[SRTEncoderSetting]("passphrase", p)
}

// SRTEncoderBitDepth sets the video bit depth (8 or 10).
func SRTEncoderBitDepth(bits int) SRTEncoderSetting { return set[SRTEncoderSetting]("vBit", bits) }

// SRTEncoderIntraRefresh enables periodic intra refresh.
func SRTEncoderIntraRefresh(on bool) SRTEncoderSetting {
	return set[SRTEncoderSetting]("vPIntraOn", on)
}

// SRTEncoderGOP sets a fixed GOP size. If on is false the GOP size is
// automatic.
func SRTEncoderGOP(on bool, size int) SRTEncoderSetting {
	return func(f *fields) {
		f.set("vGOPOn", on)
		f.set("vGOP", size)
	}
}

// SRTEncoderBitrate sets the video bitrate in Mbps.
func SRTEncoderBitrate(mbps int) SRTEncoderSetting { return set[SRTEncoderSetting]("vTBR", mbps) }

// SRTEncoderAudioBitrate sets the audio bitrate per channel in kbps.
func SRTEncoderAudioBitrate(kbps int) SRTEncoderSetting {
	return set[SRTEncoderSetting]("aTBR", kbps)
}

// SRTEncoderCard selects the video input.
func SRTEncoderCard(card VideoCard) SRTEncoderSetting {
	return set[SRTEncoderSetting]("vCard", numText(int(card)))
}

// SRTEncoderVideoMode sets the expected video standard of the input.
func SRTEncoderVideoMode(m VideoMode) SRTEncoderSetting {
	return set[SRTEncoderSetting]("vMode", string(m))
}

// SRTEncoderAudioChannels sets the number of audio channels (0-16).
func SRTEncoderAudioChannels(n int) SRTEncoderSetting { return set[SRTEncoderSetting]("aCh", n) }

// SRT decoder settings.

// SRTDecoderName renames the SRT decoder.
func SRTDecoderName(name string) SRTDecoderSetting { return set[SRTDecoderSetting]("name", name) }

// SRTDecoderAutoStart starts the SRT decoder automatically after boot.
func SRTDecoderAutoStart(on bool) SRTDecoderSetting {
	return set[SRTDecoderSetting]("autoStart", on)
}

// SRTDecoderMode sets listener or caller mode.
func SRTDecoderMode(m SRTMode) SRTDecoderSetting { return set[SRTDecoderSetting]("mode", int(m)) }

// SRTDecoderAddress sets the remote host in caller mode.
func SRTDecoderAddress(host string) SRTDecoderSetting {
	return set[SRTDecoderSetting]("address", host)
}

// SRTDecoderPort sets the remote port in caller mode.
func SRTDecoderPort(port int) SRTDecoderSetting {
	return set[SRTDecoderSetting]("port", numText(port))
}

// SRTDecoderLocalPort sets the port to listen on in listener mode, or the
// destination port in caller mode.
func SRTDecoderLocalPort(port int) SRTDecoderSetting {
	return set[SRTDecoderSetting]("localPort", numText(port))
}

// SRTDecoderFixedLocalPort uses the local port in caller mode.
func SRTDecoderFixedLocalPort(on bool) SRTDecoderSetting {
	return set[SRTDecoderSetting]("localPortOn", on)
}

// SRTDecoderLatency sets the SRT latency.
func SRTDecoderLatency(d time.Duration) SRTDecoderSetting {
	return set[SRTDecoderSetting]("latency", numText(millis(d)))
}

// SRTDecoderEncryption enables AES encryption.
func SRTDecoderEncryption(on bool) SRTDecoderSetting {
	return set[SRTDecoderSetting]("encrytion", on)
}

// SRTDecoderKeyLength sets the AES key length.
func SRTDecoderKeyLength(k SRTKeyLength) SRTDecoderSetting {
	return set[SRTDecoderSetting]("pbkeylen", int(k))
}

// SRTDecoderPassphrase sets the encryption passphrase. Note that the device
// receives it in plain text over an unencrypted websocket.
func SRTDecoderPassphrase(p string) SRTDecoderSetting {
	return set[SRTDecoderSetting]("passphrase", p)
}

// SRTDecoderCard selects the video output.
func SRTDecoderCard(card VideoCard) SRTDecoderSetting {
	return set[SRTDecoderSetting]("vCard", numText(int(card)))
}

// SRTDecoderAudio enables audio on the output.
func SRTDecoderAudio(on bool) SRTDecoderSetting { return set[SRTDecoderSetting]("audio", on) }

// SRTDecoderAudioCodec sets the expected audio codec.
func SRTDecoderAudioCodec(c SRTAudioCodec) SRTDecoderSetting {
	return set[SRTDecoderSetting]("aMode", int(c))
}

// NDI encoder settings.

// NDIEncoderName renames the NDI encoder.
func NDIEncoderName(name string) NDIEncoderSetting { return set[NDIEncoderSetting]("name", name) }

// NDIEncoderAutoStart starts the NDI encoder automatically after boot.
func NDIEncoderAutoStart(on bool) NDIEncoderSetting {
	return set[NDIEncoderSetting]("autoStart", on)
}

// NDIEncoderCard selects the SDI input.
func NDIEncoderCard(card VideoCard) NDIEncoderSetting {
	return set[NDIEncoderSetting]("vCard", numText(int(card)))
}

// NDIEncoderVideoMode sets the expected video standard of the input.
func NDIEncoderVideoMode(m VideoMode) NDIEncoderSetting {
	return set[NDIEncoderSetting]("vMode", string(m))
}

// NDIEncoderBarsStandard sets the video standard of the test pattern.
func NDIEncoderBarsStandard(m VideoMode) NDIEncoderSetting {
	return set[NDIEncoderSetting]("vModeB", string(m))
}

// NDIEncoderAudioChannels sets the number of audio channels (2, 8 or 16).
func NDIEncoderAudioChannels(n int) NDIEncoderSetting { return set[NDIEncoderSetting]("aCh", n) }

// NDI decoder settings.

// NDIDecoderName renames the NDI decoder.
func NDIDecoderName(name string) NDIDecoderSetting { return set[NDIDecoderSetting]("name", name) }

// NDIDecoderAutoStart starts the NDI decoder automatically after boot.
func NDIDecoderAutoStart(on bool) NDIDecoderSetting {
	return set[NDIDecoderSetting]("autoStart", on)
}

// NDIDecoderAVSync enables audio/video synchronisation.
func NDIDecoderAVSync(on bool) NDIDecoderSetting { return set[NDIDecoderSetting]("vAVSOn", on) }

// NDIDecoderFPSSync enables frame rate synchronisation.
func NDIDecoderFPSSync(on bool) NDIDecoderSetting { return set[NDIDecoderSetting]("vFRCOn", on) }

// NDIDecoderSDIBuffer enables the SDI output buffer.
func NDIDecoderSDIBuffer(on bool) NDIDecoderSetting {
	return set[NDIDecoderSetting]("vSDIBuffer", on)
}

// NDIDecoderAudioChannels sets the number of audio channels (2, 8 or 16).
func NDIDecoderAudioChannels(n int) NDIDecoderSetting { return set[NDIDecoderSetting]("aCh", n) }

// NDIDecoderCard selects the SDI output.
func NDIDecoderCard(card VideoCard) NDIDecoderSetting {
	return set[NDIDecoderSetting]("vCard", numText(int(card)))
}

// NDIDecoderVideoMode sets the video standard of the output.
func NDIDecoderVideoMode(m VideoMode) NDIDecoderSetting {
	return set[NDIDecoderSetting]("vMode", string(m))
}

// NDIDecoderSDILevelA selects SDI level A instead of B.
func NDIDecoderSDILevelA(on bool) NDIDecoderSetting {
	return set[NDIDecoderSetting]("sdilevelA", boolText(on))
}

// NDIDecoderProgressiveNotPsF outputs 1080p instead of PsF.
func NDIDecoderProgressiveNotPsF(on bool) NDIDecoderSetting {
	return set[NDIDecoderSetting]("use1080pNotPsf", on)
}

// NDIDecoderResize enables the "resize" option of the NDI decoder (key
// "vResize").
func NDIDecoderResize(on bool) NDIDecoderSetting { return set[NDIDecoderSetting]("vResize", on) }
