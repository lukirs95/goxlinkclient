package model

import "strconv"

type EncoderReceiver interface {
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

type EncoderRawReceiver struct {
	Id     string    `json:"id"`
	Name   string         `json:"name"`
	Values *DecoderRawValues `json:"values"`
}

func (receiver EncoderRawReceiver) Ident() string {
	return receiver.Id
}

func (receiver EncoderRawReceiver) GetName() (string, bool) {
	if receiver.Name != "" {
		return receiver.Name, true
	}
	return "", false
}

func (receiver EncoderRawReceiver) PhyicalNumber() (int, bool) {
	if receiver.Values != nil && receiver.Values.VCard != "" {
		num, err := strconv.Atoi(receiver.Values.VCard)
		if err != nil {
			return 0, false
		}
		return num, true
	}
	return 0, false
}

func (receiver EncoderRawReceiver) IsVideoEnabled() (bool, bool) {
	if receiver.Values != nil && receiver.Values.Video2110Enabled != nil {
		return *receiver.Values.Video2110Enabled, true
	}
	return false, false
}

func (receiver EncoderRawReceiver) IsAudioEnabled() (bool, bool) {
	if receiver.Values != nil && receiver.Values.Audio2110Enabled != nil {
		return *receiver.Values.Audio2110Enabled, true
	}
	if receiver.Values != nil && receiver.Values.AudioSDIEnabled != nil {
		return *receiver.Values.AudioSDIEnabled, true
	}
	return false, false
}

func (receiver EncoderRawReceiver) HasVideoSignal() (bool, bool) {
	if receiver.Values != nil && receiver.Values.VOut != "" {
		return receiver.Values.VOut != "No Signal", true
	}
	return false, false
}

func (receiver EncoderRawReceiver) HasAudioSignal() (bool, bool) {
	if receiver.Values != nil && receiver.Values.AOut != "" {
		return receiver.Values.AOut != "No Signal", true
	}
	return false, false
}

func (receiver EncoderRawReceiver) IsRunning() (bool, bool) {
	if receiver.Values != nil && receiver.Values.Running != nil {
		return *receiver.Values.Running, true
	}
	return false, false
}

func (receiver EncoderRawReceiver) IsConnected() (bool, bool) {
	if receiver.Values != nil && receiver.Values.Connected != nil && receiver.Values.XLinkP2P != nil {
		return *receiver.Values.Connected && *receiver.Values.XLinkP2P, true
	}
	return false, false
}
