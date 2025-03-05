package model

import "strconv"

type DecoderSender interface {
	Ident() string
	GetName() (string, bool)
	PhyicalNumber() (int, bool)
	IsVideoEnabled() (bool, bool)
	IsAudioEnabled() (bool, bool)
	HasVideoSignal() (bool, bool)
	HasAudioSignal() (bool, bool)
	IsRunning() (bool, bool)
	IsConnected() (bool, bool)
}

type DecoderRawSender struct {
	Id     string            `json:"id"`
	Name   string            `json:"name"`
	Values *DecoderRawValues `json:"values"`
}

func (sender DecoderRawSender) Ident() string {
	return sender.Id
}

func (sender DecoderRawSender) GetName() (string, bool) {
	if sender.Name != "" {
		return sender.Name, true
	}
	return "", false
}

func (sender DecoderRawSender) PhyicalNumber() (int, bool) {
	if sender.Values != nil && sender.Values.VCard != "" {
		num, err := strconv.Atoi(sender.Values.VCard)
		if err != nil {
			return 0, false
		}
		return num, true
	}
	return 0, false
}

func (sender DecoderRawSender) IsVideoEnabled() (bool, bool) {
	if sender.Values != nil && sender.Values.Video2110Enabled != nil {
		return *sender.Values.Video2110Enabled, true
	}
	return false, false
}

func (sender DecoderRawSender) IsAudioEnabled() (bool, bool) {
	if sender.Values != nil && sender.Values.AudioSDIEnabled != nil {
		return *sender.Values.AudioSDIEnabled, true
	}
	if sender.Values != nil && sender.Values.AudioSDIEnabled != nil {
		return *sender.Values.AudioSDIEnabled, true
	}
	return false, false
}

func (sender DecoderRawSender) HasVideoSignal() (bool, bool) {
	if sender.Values != nil && sender.Values.VIn != "" {
		return sender.Values.VIn != "No Signal", true
	}
	return false, false
}

func (sender DecoderRawSender) HasAudioSignal() (bool, bool) {
	if sender.Values != nil && sender.Values.AIn != "" {
		return sender.Values.AIn != "No Signal", true
	}
	return false, false
}

func (sender DecoderRawSender) IsRunning() (bool, bool) {
	if sender.Values != nil && sender.Values.Running != nil {
		return *sender.Values.Running, true
	}
	return false, false
}

func (sender DecoderRawSender) IsConnected() (bool, bool) {
	if sender.Values != nil && sender.Values.XLinkP2P != nil {
		return *sender.Values.Connected && *sender.Values.XLinkP2P, true
	}
	return false, false
}
