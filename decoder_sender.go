package xlinkclient

import "strconv"

type DecoderSender struct {
	Id     string         `json:"id"`
	Name   string         `json:"name"`
	Values *DecoderValues `json:"values"`
}

func (sender DecoderSender) Ident() string {
	return sender.Id
}

func (sender DecoderSender) GetName() (string, bool) {
	if sender.Name != "" {
		return sender.Name, true
	}
	return "", false
}

func (sender DecoderSender) PhyicalNumber() (int, bool) {
	if sender.Values != nil && sender.Values.VCard != "" {
		num, err := strconv.Atoi(sender.Values.VCard)
		if err != nil {
			return 0, false
		}
		return num, true
	}
	return 0, false
}

func (sender DecoderSender) IsVideoEnabled() (bool, bool) {
	if sender.Values != nil && sender.Values.Video2110Enabled != nil {
		return *sender.Values.Video2110Enabled, true
	}
	return false, false
}

func (sender DecoderSender) IsAudioEnabled() (bool, bool) {
	if sender.Values != nil && sender.Values.AudioSDIEnabled != nil {
		return *sender.Values.AudioSDIEnabled, true
	}
	if sender.Values != nil && sender.Values.AudioSDIEnabled != nil {
		return *sender.Values.AudioSDIEnabled, true
	}
	return false, false
}

func (sender DecoderSender) HasVideoSignal() (bool, bool) {
	if sender.Values != nil && sender.Values.VIn != "" {
		return sender.Values.VIn != "No Signal", true
	}
	return false, false
}

func (sender DecoderSender) HasAudioSignal() (bool, bool) {
	if sender.Values != nil && sender.Values.AIn != "" {
		return sender.Values.AIn != "No Signal", true
	}
	return false, false
}

func (sender DecoderSender) IsRunning() (bool, bool) {
	if sender.Values != nil && sender.Values.Running != nil {
		return *sender.Values.Running, true
	}
	return false, false
}

func (sender DecoderSender) IsConnected() (bool, bool) {
	if sender.Values != nil && sender.Values.XLinkP2P != nil {
		return *sender.Values.Connected && *sender.Values.XLinkP2P, true
	}
	return false, false
}
