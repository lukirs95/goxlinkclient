package xlinkclient

type XLink interface {
	Ident() string
	GetName() string
	GetEncoders() []Encoder
	GetDecoders() []Decoder
	GetInterfaces() []Ethernet
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
	GetReceiver() (EncoderReceiver, bool)
}

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
}

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