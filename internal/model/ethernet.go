package model

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

type EthernetRaw struct {
	Id         string `json:"id"`
	Ip         string `json:"ip"`
	Gate       string `json:"gate"`
	Mask       string `json:"mask"`
	Link       *bool  `json:"link"`
	Admin      *bool  `json:"admin"`
	Enabled    *bool  `json:"enabled"`
	DefaultLan *bool  `json:"defaultLan"`
	AdminOnly  *bool  `json:"adminOnly"`
	Igmp       *bool  `json:"igmp"`
	Ndi        *bool  `json:"ndi"`
	Default    *bool  `json:"default"`
	Backup     *bool  `json:"backup"`
	Active     *bool  `json:"active"`
}

func (eth EthernetRaw) Ident() string {
	return eth.Id
}
func (eth EthernetRaw) IPAddress() (string, bool) {
	if eth.Ip != "" {
		return eth.Ip, true
	}
	return "", false
}
func (eth EthernetRaw) Gateway() (string, bool) {
	if eth.Gate != "" {
		return eth.Gate, true
	}
	return "", false
}
func (eth EthernetRaw) SubnetMask() (string, bool) {
	if eth.Mask != "" {
		return eth.Mask, true
	}
	return "", false
}
func (eth EthernetRaw) IsLinkUp() (bool, bool) {
	if eth.Link != nil {
		return *eth.Link, true
	}
	return false, false
}
func (eth EthernetRaw) IsEnabled() (bool, bool) {
	if eth.Enabled != nil {
		return *eth.Enabled, true
	}
	return false, false
}
func (eth EthernetRaw) IsDefaultLan() (bool, bool) {
	if eth.DefaultLan != nil {
		return *eth.DefaultLan, true
	}
	return false, false
}
func (eth EthernetRaw) IsAdminOnly() (bool, bool) {
	if eth.AdminOnly != nil {
		return *eth.AdminOnly, true
	}
	return false, false
}
func (eth EthernetRaw) IsDefaultUplink() (bool, bool) {
	if eth.Default != nil {
		return *eth.Default, true
	}
	return false, false
}
func (eth EthernetRaw) IsBackupUplink() (bool, bool) {
	if eth.Backup != nil {
		return *eth.Backup, true
	}
	return false, false
}
func (eth EthernetRaw) IsActive() (bool, bool) {
	if eth.Active != nil {
		return *eth.Active, true
	}
	return false, false
}