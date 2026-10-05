package xlinkclient

import "time"

// Interface is a network interface of a system.
type Interface struct {
	// ID is the interface name, e.g. "eth0".
	ID      string
	MAC     string
	DHCP    bool
	IP      string
	Mask    string
	Gateway string
	DNS     [2]string
	Enabled bool
	LinkUp  bool
	// LinkSince is when the link came up, or the zero time if it never did.
	LinkSince time.Time
	// Speed is the link speed as reported by the device, e.g. "25Gbps" or "?".
	Speed string
	// Active reports whether the interface currently carries traffic.
	Active bool
	// Internet reports whether the internet is reachable via this interface.
	Internet bool
	// GatewayPing reports whether the gateway answers pings.
	GatewayPing bool
	// AdminOnly restricts the interface to the web admin.
	AdminOnly bool
	// WebAdmin allows access to the web admin via this interface.
	WebAdmin bool
	// HTTPSOnly restricts the web admin to HTTPS.
	HTTPSOnly bool
	IGMP      bool
	NMOS      bool
	// DefaultExternal marks the default uplink ("Default External").
	DefaultExternal bool
	// DefaultLAN marks the default LAN for NDI ("Lan (NDI)").
	DefaultLAN bool
	// Backup marks the backup uplink ("Backup External").
	Backup bool
	// Trunk is the layer 2 trunk using this interface, or "" if none.
	Trunk TrunkID
}

// Trunk is an XLink layer 2 trunk.
type Trunk struct {
	ID      TrunkID
	Name    string
	Running bool
	// Interface is the network interface the trunk is bridged to.
	Interface         string
	Master            bool
	AutoStart         bool
	Multicast         bool
	ReliableMulticast bool
	Encryption        bool
	// MTU overrides the system trunk MTU. Zero means the system default.
	MTU int
}
