package model

type XLink interface {
	Ident() string
	GetName() string
	GetEncoders() []Encoder
	GetDecoders() []Decoder
	GetInterfaces() []Ethernet
}

type XLinkRaw struct {
	Id   string `json:"sysid"`
	Data struct {
		Local Local `json:"local"`
	} `json:"data"`
}

func (xLink XLinkRaw) Ident() string {
	return xLink.Id
}

func (xLink XLinkRaw) GetName() string {
	return xLink.Data.Local.Name
}

func (xlink XLinkRaw) GetEncoders() []Encoder {
	encoders := make([]Encoder, 0)
	for _, encoder := range xlink.Data.Local.Enc {
		encoders = append(encoders, encoder)
	}
	return encoders
}

func (xlink XLinkRaw) GetDecoders() []Decoder {
	decoders := make([]Decoder, 0)
	for _, decoder := range xlink.Data.Local.Dec {
		decoders = append(decoders, decoder)
	}
	return decoders
}

func (xlink XLinkRaw) GetInterfaces() []Ethernet {
	eths := make([]Ethernet, 0)
	for _, eth := range xlink.Data.Local.Network.Nets {
		eths = append(eths, eth)
	}
	return eths
}

type Local struct {
	Name    string    `json:"name"`
	Enc     []EncoderRaw `json:"enc"`
	Dec     []DecoderRaw `json:"dec"`
	Network struct {
		Nets []EthernetRaw `json:"nets"`
	} `json:"network"`
}