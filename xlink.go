package xlinkclient

type XLink struct {
	Id   string `json:"sysid"`
	Data struct {
		Local XLinkLocal `json:"local"`
	} `json:"data"`
}

type XLinkLocal struct {
	Name    string     `json:"name"`
	Enc     []*Encoder `json:"enc"`
	Dec     []*Decoder `json:"dec"`
	Network struct {
		Nets []*Ethernet `json:"nets"`
	} `json:"network"`
}

func (xLink XLink) Ident() string {
	return xLink.Id
}

func (xLink XLink) GetName() (string, bool) {
	if xLink.Data.Local.Name != "" {
		return xLink.Data.Local.Name, true
	}
	return "", false
}

func (xlink XLink) GetEncoders() []*Encoder {
	return xlink.Data.Local.Enc
}

func (xlink XLink) GetDecoders() []*Decoder {
	return xlink.Data.Local.Dec
}

func (xlink XLink) GetInterfaces() []*Ethernet {
	return xlink.Data.Local.Network.Nets
}
