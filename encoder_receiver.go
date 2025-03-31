package xlinkclient

import "strconv"

type EncoderReceiver struct {
	Id     string         `json:"id"`
	Name   string         `json:"name"`
	Values *DecoderValues `json:"values"`
}

func (receiver EncoderReceiver) Ident() string {
	return receiver.Id
}

func (receiver EncoderReceiver) GetName() (string, bool) {
	if receiver.Name != "" {
		return receiver.Name, true
	}
	return "", false
}

func (receiver EncoderReceiver) PhyicalNumber() (int, bool) {
	if receiver.Values != nil && receiver.Values.VCard != "" {
		num, err := strconv.Atoi(receiver.Values.VCard)
		if err != nil {
			return 0, false
		}
		return num, true
	}
	return 0, false
}

func (receiver EncoderReceiver) IsVideoEnabled() (bool, bool) {
	if receiver.Values != nil && receiver.Values.Video2110Enabled != nil {
		return *receiver.Values.Video2110Enabled, true
	}
	return false, false
}

func (receiver EncoderReceiver) IsAudioEnabled() (bool, bool) {
	if receiver.Values != nil && receiver.Values.Audio2110Enabled != nil {
		return *receiver.Values.Audio2110Enabled, true
	}
	if receiver.Values != nil && receiver.Values.AudioSDIEnabled != nil {
		return *receiver.Values.AudioSDIEnabled, true
	}
	return false, false
}

func (receiver EncoderReceiver) HasVideoSignal() (bool, bool) {
	if receiver.Values != nil && receiver.Values.VOut != "" {
		return receiver.Values.VOut != "No Signal", true
	}
	return false, false
}

func (receiver EncoderReceiver) HasAudioSignal() (bool, bool) {
	if receiver.Values != nil && receiver.Values.AOut != "" {
		return receiver.Values.AOut != "No Signal", true
	}
	return false, false
}

func (receiver EncoderReceiver) IsRunning() (bool, bool) {
	if receiver.Values != nil && receiver.Values.Running != nil {
		return *receiver.Values.Running, true
	}
	return false, false
}

func (receiver EncoderReceiver) IsConnected() (bool, bool) {
	if receiver.Values != nil && receiver.Values.XLinkP2P != nil {
		return *receiver.Values.Connected && *receiver.Values.XLinkP2P, true
	}
	return false, false
}
