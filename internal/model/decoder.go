package model

import "strconv"

type Decoder interface {
	Ident() string
	GetName() (string, bool)
	IsEnabled() (bool, bool)
	PhyicalNumber() (int, bool)
	IsVideoEnabled() (bool, bool)
	IsAudioEnabled() (bool, bool)
	HasVideoSignal() (bool, bool)
	HasAudioSignal() (bool, bool)
	IsRunning() (bool, bool)
	HasSender() (bool, bool)
	IsConnected() (bool, bool)
	GetSender() (DecoderSender, bool)
}

type DecoderRaw struct {
	Id      string            `json:"id"`
	Enabled *bool             `json:"enabled"`
	Name    string            `json:"name"`
	Values  *DecoderRawValues `json:"values"`
	Sender  *DecoderRawSender `json:"sender"`
}

func (decoder DecoderRaw) Ident() string {
	return decoder.Id
}

func (decoder DecoderRaw) GetName() (string, bool) {
	if decoder.Name != "" {
		return decoder.Name, true
	}
	return "", false
}

func (decoder DecoderRaw) IsEnabled() (bool, bool) {
	if decoder.Enabled != nil {
		return *decoder.Enabled, true
	}
	return false, false
}

func (decoder DecoderRaw) PhyicalNumber() (int, bool) {
	if decoder.Values != nil && decoder.Values.VCard != "" {
		num, err := strconv.Atoi(decoder.Values.VCard)
		if err != nil {
			return 0, false
		}
		return num, true
	}
	return 0, false
}

func (decoder DecoderRaw) IsVideoEnabled() (bool, bool) {
	if decoder.Values != nil && decoder.Values.Video2110Enabled != nil {
		return *decoder.Values.Video2110Enabled, true
	}
	return false, false
}

func (decoder DecoderRaw) IsAudioEnabled() (bool, bool) {
	if decoder.Values != nil && decoder.Values.Audio2110Enabled != nil {
		return *decoder.Values.Audio2110Enabled, true
	}
	if decoder.Values != nil && decoder.Values.AudioSDIEnabled != nil {
		return *decoder.Values.AudioSDIEnabled, true
	}
	return false, false
}

func (decoder DecoderRaw) HasVideoSignal() (bool, bool) {
	if decoder.Values != nil && decoder.Values.VOut != "" {
		return decoder.Values.VOut != "No Signal", true
	}
	return false, false
}

func (decoder DecoderRaw) HasAudioSignal() (bool, bool) {
	if decoder.Values != nil && decoder.Values.AOut != "" {
		return decoder.Values.AOut != "No Signal", true
	}
	return false, false
}

func (decoder DecoderRaw) IsRunning() (bool, bool) {
	if decoder.Values != nil && decoder.Values.Running != nil {
		return *decoder.Values.Running, true
	}
	return false, false
}

func (decoder DecoderRaw) HasSender() (bool, bool) {
	if decoder.Sender != nil && decoder.Sender.Id != "" {
		return decoder.Sender.Id != "none", true
	}
	return false, false
}

func (decoder DecoderRaw) IsConnected() (bool, bool) {
	if decoder.Sender != nil {
		return decoder.Sender.IsConnected()
	}
	return false, false
}

func (decoder DecoderRaw) GetSender() (DecoderSender, bool) {
	if _, OK := decoder.HasSender(); OK {
		return decoder.Sender, true
	}
	return nil, false
}

type DecoderRawValues struct {
	VIn              string `json:"vIn"`
	VOut             string `json:"vOut"`
	AIn              string `json:"aIn"`
	AOut             string `json:"aOut"`
	VCard            string `json:"vCard"`
	Video2110Enabled *bool  `json:"v2110NetPriEnabled"`
	Audio2110Enabled *bool  `json:"a2110NetPriEnabled"`
	AudioSDIEnabled  *bool  `json:"audio"`
	Connected        *bool  `json:"connected"`
	Running          *bool  `json:"running"`
	XLinkP2P         *bool  `json:"xLinkp2p"`
}
