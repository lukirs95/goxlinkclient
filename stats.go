package xlinkclient

import "github.com/lukirs95/goxlinkclient/internal/model"

type Stats interface {
	Id() string
	SystemStats() model.SystemStats
	EthStats() []model.EthStats
	EncoderStats() []model.EncoderStats
	DecoderStats() []model.DecoderStats
}

type SystemStats interface {
	Ident() string
	PtpSync() bool
	PtpSyncLocal() bool
	Ptp() bool
	OSUpTime() int64
	CPUTemp() int
	SysTemp() int
}

type EthStats interface {
	Ident() string
	RX() float32
	TX() float32
}

type DecoderStats interface {
	Ident() string
	RTT() float32
	UpTime() int64
	StatsTime() int64
	FromCloud() int64
	FromP2P() int64
	Dropped() int64
	Resent() int64
	ResentDropped() int64
	VideoDTotal() int64
	VideoDDrop() int64
	VideoDCorr() int64
	VideoDMissing() int64
	VideoRMissing() int64
	VideoOutFps() float32
	RXmbps() float32
	TXmbps() float32
}

type EncoderStats interface {
	Ident() string
	UpTime() int64
	StatsTime() int64
	VideoInFps() float32
}