package model

import "strconv"

type Encoder interface {
	Ident() string
	GetName() (string, bool)
	IsEnabled() (bool, bool)
	PhyicalNumber() (int, bool)
	IsVideoEnabled() (bool, bool)
	IsAudioEnabled() (bool, bool)
	IsRunning() (bool, bool)
	IsConnected() (bool, bool)
	GetReceiver() (EncoderReceiver, bool)
}

type EncoderRaw struct {
	Id       string      `json:"id"`
	Enabled  *bool            `json:"enabled"`
	Name     string           `json:"name"`
	Values   *EncoderRawValues   `json:"values"`
	Receiver *EncoderRawReceiver `json:"receiver"`
}

func (encoder EncoderRaw) Ident() string {
	return encoder.Id
}

func (encoder EncoderRaw) GetName() (string, bool) {
	if encoder.Name != "" {
		return encoder.Name, true
	}
	return "", false
}

func (encoder EncoderRaw) IsEnabled() (bool, bool) {
	if encoder.Enabled != nil {
		return *encoder.Enabled, true
	}
	return false, false
}

func (encoder EncoderRaw) PhyicalNumber() (int, bool) {
	if encoder.Values != nil && encoder.Values.VCard != "" {
		num, err := strconv.Atoi(encoder.Values.VCard)
		if err != nil {
			return 0, false
		}
		return num, true
	}
	return 0, false
}

func (encoder EncoderRaw) IsVideoEnabled() (bool, bool) {
	if encoder.Values != nil && encoder.Values.Video2110Enabled != nil {
		return *encoder.Values.Video2110Enabled, true
	}
	return false, false
}

func (encoder EncoderRaw) IsAudioEnabled() (bool, bool) {
	if encoder.Values != nil && encoder.Values.Audio2110Enabled != nil {
		return *encoder.Values.Audio2110Enabled, true
	}
	if encoder.Values != nil && encoder.Values.AudioSDIEnabled != nil {
		return *encoder.Values.AudioSDIEnabled, true
	}
	return false, false
}

func (encoder EncoderRaw) HasVideoSignal() (bool, bool) {
	if encoder.Values != nil && encoder.Values.VIn != "" {
		return encoder.Values.VIn != "No Signal", true
	}
	return false, false
}

func (encoder EncoderRaw) HasAudioSignal() (bool, bool) {
	if encoder.Values != nil && encoder.Values.AIn != "" {
		return encoder.Values.AIn != "No Signal", true
	}
	return false, false
}

func (encoder EncoderRaw) IsRunning() (bool, bool) {
	if encoder.Values != nil && encoder.Values.Running != nil {
		return *encoder.Values.Running, true
	}
	return false, false
}

func (encoder EncoderRaw) HasReceiver() (bool, bool) {
	if encoder.Receiver != nil && encoder.Receiver.Id != "" {
		return encoder.Receiver.Id != "none", true
	}
	return false, false
}

func (encoder EncoderRaw) IsConnected() (bool, bool) {
	if encoder.Receiver != nil {
		return encoder.Receiver.IsConnected()
	}
	return false, false
}

func (encoder EncoderRaw) GetReceiver() (EncoderReceiver, bool) {
	if _, OK := encoder.HasReceiver(); OK {
		return encoder.Receiver, true
	}
	return nil, false
}

type EncoderRawValues struct {
	VIn              string `json:"vIn"`
	AIn              string `json:"aIn"`
	VCard            string `json:"vCard"`
	Video2110Enabled *bool  `json:"v2110NetPriEnabled"`
	Audio2110Enabled *bool  `json:"a2110NetPriEnabled"`
	AudioSDIEnabled  *bool  `json:"audio"`
	Connected        *bool  `json:"connected"`
	Running          *bool  `json:"running"`
	XLinkP2P         *bool  `json:"xLinkp2p"`
}
