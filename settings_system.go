package xlinkclient

import "context"

// InterfaceSetting changes a setting of a network interface. See
// ConfigureInterface.
type InterfaceSetting func(*fields)

// PTPSetting changes a PTP setting. See ConfigurePTP.
type PTPSetting func(*fields)

// NMOSSetting changes an NMOS setting. See ConfigureNMOS.
type NMOSSetting func(*fields)

// DNSSetting changes the system DNS setting. See ConfigureDNS.
type DNSSetting func(*fields)

// TrunkSetting changes a setting of a layer 2 trunk. See ConfigureTrunk.
type TrunkSetting func(*fields)

// ConfigureInterface changes settings of a network interface of the local
// system, e.g. "eth3". All settings are sent in one request.
func (c *Client) ConfigureInterface(ctx context.Context, name string, settings ...InterfaceSetting) error {
	f := collect(settings)
	if len(f) == 0 {
		return nil
	}
	values := f.object()
	values["eth"] = name
	return c.callSystem(ctx, methodConfigEth, values)
}

// ConfigurePTP changes PTP settings of the local system. The settings are sent
// one request each, in order, as the web UI does.
func (c *Client) ConfigurePTP(ctx context.Context, settings ...PTPSetting) error {
	return c.callSystemEach(ctx, methodSet2110, collect(settings))
}

// ConfigureNMOS changes NMOS settings of the local system. The settings are
// sent one request each, in order, as the web UI does.
func (c *Client) ConfigureNMOS(ctx context.Context, settings ...NMOSSetting) error {
	return c.callSystemEach(ctx, methodSet2110, collect(settings))
}

// ConfigureDNS changes the system wide DNS setting. The settings are sent one
// request each, in order, as the web UI does.
func (c *Client) ConfigureDNS(ctx context.Context, settings ...DNSSetting) error {
	return c.callSystemEach(ctx, methodDNS, collect(settings))
}

// ConfigureTrunk changes settings of a layer 2 trunk, which may belong to a
// remote system. The settings are sent one request each, in order, as the web
// UI does.
func (c *Client) ConfigureTrunk(ctx context.Context, id TrunkID, settings ...TrunkSetting) error {
	for _, kv := range collect(settings) {
		values := map[string]any{kv.key: kv.value}
		if err := c.callUnit(ctx, methodConfigTrunk, id.System(), string(id), values); err != nil {
			return err
		}
	}
	return nil
}

// SetName renames the local system.
func (c *Client) SetName(ctx context.Context, name string) error {
	return c.callSystem(ctx, methodSystemName, map[string]any{"name": name})
}

// SetSystemPort sets the XLink system port. If static is false the device
// picks the port and port is ignored. The system must be restarted to apply
// the change.
func (c *Client) SetSystemPort(ctx context.Context, static bool, port int) error {
	values := map[string]any{"sysOn": static}
	if static {
		values["sysPort"] = numText(port)
	}
	return c.callSystem(ctx, methodSystemPorts, values)
}

// SetDataPorts sets the XLink data port range. If static is false the device
// picks the ports and from and to are ignored. The system must be restarted to
// apply the change.
func (c *Client) SetDataPorts(ctx context.Context, static bool, from, to int) error {
	values := map[string]any{"portsOn": static}
	if static {
		values["portsFrom"] = numText(from)
		values["portsTo"] = numText(to)
	}
	return c.callSystem(ctx, methodSystemPorts, values)
}

// SetTrunkMTU sets the default MTU of layer 2 trunks.
func (c *Client) SetTrunkMTU(ctx context.Context, mtu int) error {
	return c.callSystem(ctx, methodSystemMTU, map[string]any{"mtuTrunk": mtu})
}

// AddPeer adds a remote system to the configured remote systems. It must be
// on the running profile.
func (c *Client) AddPeer(ctx context.Context, id SystemID) error {
	return c.callSystem(ctx, methodAddPeer, map[string]any{"systemId": string(id)})
}

// SetManualIP sets how the local system connects to a remote system. The zero
// ManualIP clears the setting.
func (c *Client) SetManualIP(ctx context.Context, peer SystemID, m ManualIP) error {
	port := ""
	if m.Port != 0 {
		port = numText(m.Port)
	}
	return c.callSystem(ctx, methodManualIP, map[string]any{
		"peer":      string(peer),
		"manIp":     m.Primary,
		"manIpSec":  m.Secondary,
		"manPort":   port,
		"manAutCon": m.AutoConnect,
	})
}

// Network interface settings.

// InterfaceEnabled enables the interface.
func InterfaceEnabled(on bool) InterfaceSetting { return set[InterfaceSetting]("enabled", on) }

// InterfaceDefaultExternal makes the interface the default uplink.
func InterfaceDefaultExternal(on bool) InterfaceSetting {
	return set[InterfaceSetting]("default", on)
}

// InterfaceDefaultLAN makes the interface the default LAN for NDI.
func InterfaceDefaultLAN(on bool) InterfaceSetting {
	return set[InterfaceSetting]("defaultLan", on)
}

// InterfaceBackup makes the interface the backup uplink.
func InterfaceBackup(on bool) InterfaceSetting { return set[InterfaceSetting]("backup", on) }

// InterfaceWebAdmin allows access to the web admin via the interface.
func InterfaceWebAdmin(on bool) InterfaceSetting { return set[InterfaceSetting]("admin", on) }

// InterfaceHTTPSOnly restricts the web admin to HTTPS.
func InterfaceHTTPSOnly(on bool) InterfaceSetting {
	return set[InterfaceSetting]("adminSslOnly", on)
}

// InterfaceIGMP enables IGMP on the interface.
func InterfaceIGMP(on bool) InterfaceSetting { return set[InterfaceSetting]("igmp", on) }

// InterfaceNMOS enables NMOS on the interface.
func InterfaceNMOS(on bool) InterfaceSetting { return set[InterfaceSetting]("nmos", on) }

// InterfaceDHCP configures the interface by DHCP.
func InterfaceDHCP() InterfaceSetting { return set[InterfaceSetting]("dhcp", true) }

// InterfaceStaticIP configures a static address.
func InterfaceStaticIP(ip, mask, gateway string, dns [2]string) InterfaceSetting {
	return func(f *fields) {
		f.set("dhcp", false)
		f.set("ip", ip)
		f.set("mask", mask)
		f.set("gate", gateway)
		f.set("dns1", dns[0])
		f.set("dns2", dns[1])
	}
}

// PTP settings.

// PTPEnabled enables PTP.
func PTPEnabled(on bool) PTPSetting { return set[PTPSetting]("ptp", on) }

// PTPInterface sets the interface used for PTP. An empty name clears it.
func PTPInterface(name string) PTPSetting { return set[PTPSetting]("ptpEth", ifaceRef(name)) }

// PTPDomain sets the PTP domain number.
func PTPDomain(n int) PTPSetting { return set[PTPSetting]("ptpDomainNumber", numText(n)) }

// PTPHybridE2E enables hybrid end-to-end mode.
func PTPHybridE2E(on bool) PTPSetting { return set[PTPSetting]("ptpHybrid_e2e", on) }

// PTPAnnounceInterval sets the log2 announce interval, e.g. -3.
func PTPAnnounceInterval(n int) PTPSetting {
	return set[PTPSetting]("ptpLogAnnounceInterval", numText(n))
}

// PTPAnnounceReceiptTimeout sets the announce receipt timeout.
func PTPAnnounceReceiptTimeout(n int) PTPSetting {
	return set[PTPSetting]("ptpAnnounceReceiptTimeout", numText(n))
}

// PTPMinDelayReqInterval sets the log2 minimum delay request interval.
func PTPMinDelayReqInterval(n int) PTPSetting {
	return set[PTPSetting]("ptpLogMinDelayReqInterval", numText(n))
}

// PTPDSCP sets the DSCP value of PTP packets.
func PTPDSCP(n int) PTPSetting { return set[PTPSetting]("ptpDscp", numText(n)) }

// NMOS settings.

// NMOSEnabled enables NMOS.
func NMOSEnabled(on bool) NMOSSetting { return set[NMOSSetting]("nmos", on) }

// NMOSInterface sets the interface used for NMOS. An empty name clears it.
func NMOSInterface(name string) NMOSSetting { return set[NMOSSetting]("nmosEth", ifaceRef(name)) }

// DNS settings.

// DNSStatic uses the system DNS servers on all interfaces instead of the
// per interface configuration.
func DNSStatic(on bool) DNSSetting { return set[DNSSetting]("dnsStatic", on) }

// DNSServers sets the system DNS servers.
func DNSServers(primary, secondary string) DNSSetting {
	return func(f *fields) {
		f.set("dnsStaticIp1", primary)
		f.set("dnsStaticIp2", secondary)
	}
}

// Trunk settings.

// TrunkInterface bridges the trunk to the given interface.
func TrunkInterface(name string) TrunkSetting { return set[TrunkSetting]("eth", name) }

// TrunkMaster makes this side the master of the trunk.
func TrunkMaster(on bool) TrunkSetting { return set[TrunkSetting]("master", on) }

// TrunkMTU overrides the system trunk MTU. Zero restores the default.
func TrunkMTU(mtu int) TrunkSetting {
	return func(f *fields) {
		f.set("l2mtuOn", mtu != 0)
		if mtu != 0 {
			f.set("l2mtu", mtu)
		}
	}
}

// TrunkAutoStart starts the trunk automatically after boot.
func TrunkAutoStart(on bool) TrunkSetting { return set[TrunkSetting]("autoStart", on) }

// TrunkEncryption enables encryption.
func TrunkEncryption(on bool) TrunkSetting { return set[TrunkSetting]("encryption", on) }

// TrunkMulticast enables multicast forwarding.
func TrunkMulticast(on bool) TrunkSetting { return set[TrunkSetting]("multicast", on) }

// TrunkReliableMulticast enables reliable multicast forwarding.
func TrunkReliableMulticast(on bool) TrunkSetting { return set[TrunkSetting]("rmcast", on) }

// TrunkAddPeer links a trunk of a remote system to this trunk. The remote
// trunk must exist; see CreateTrunk.
func TrunkAddPeer(remote TrunkID) TrunkSetting { return set[TrunkSetting]("addPeer", string(remote)) }

// TrunkRemovePeer unlinks a trunk of a remote system.
func TrunkRemovePeer(remote TrunkID) TrunkSetting {
	return set[TrunkSetting]("delPeer", string(remote))
}
