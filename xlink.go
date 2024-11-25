package xlinkclient

import "github.com/lukirs95/goxlinkclient/internal/model"

type XLink interface {
	Ident() string
	GetName() string
	GetEncoders() []model.Encoder
	GetDecoders() []model.Decoder
	GetInterfaces() []model.Ethernet
}

type Encoder interface {
	Ident() string
	GetName() (string, bool)
	IsEnabled() (bool, bool)
	PhyicalNumber() (int, bool)
	IsVideoEnabled() (bool, bool)
	IsAudioEnabled() (bool, bool)
	IsRunning() (bool, bool)
	IsConnected() (bool, bool)
	GetReceiver() (model.EncoderReceiver, bool)
}

type Decoder interface {
	Ident() string
	GetName() (string, bool)
	IsEnabled() (bool, bool)
	PhyicalNumber() (int, bool)
	IsVideoEnabled() (bool, bool)
	IsAudioEnabled() (bool, bool)
	IsRunning() (bool, bool)
	HasSender() (bool, bool)
	IsConnected() (bool, bool)
	GetSender() (model.DecoderSender, bool)
}

type Ethernet interface {
	Ident() string
	IPAddress() (string, bool)
	Gateway() (string, bool)
	SubnetMask() (string, bool)
	IsLinkUp() (bool, bool)
	IsEnabled() (bool, bool)
	IsDefaultLan() (bool, bool)
	IsAdminOnly() (bool, bool)
	IsDefaultUplink() (bool, bool)
	IsBackupUplink() (bool, bool)
	IsActive() (bool, bool)
}