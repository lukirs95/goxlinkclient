# VideoXLink Web-UI – Konfiguration & JSON-RPC (FW 1.8.4.6)

Reverse-Engineering-Protokoll des Web-Frontends (PrimeVue-App) eines X8-R2-Systems mit
Firmware **1.8.4.6**. Erkundet wurde **nur lesend**: Dialoge geöffnet, Tabs gewechselt,
nichts geändert oder gespeichert. Welche Requests beim *Ändern* einer Einstellung gesendet
werden, ist daher noch offen (siehe [Offene Punkte](#offene-punkte)).

Transport: WebSocket `ws://<host>/jsonrpc`, JSON-RPC 2.0.

---

## 1. Session-Ablauf

| Schritt | Richtung | Methode | Params | Antwort / Bemerkung |
|---|---|---|---|---|
| Login | → | `auth` | `{auth:true, userid, pass}` | `result: {response, userId, authKey, firstName, lastName, role, defaultPass, beta, method}` |
| Stats abonnieren | → | `localStats.subscribe` | `{sysid:"local", batch:60, max:600}` | `{method, response}` |
| | ← | `systems.localStatsHistory` | `{sysid, startTime, endTime, data}` | mehrere Blöcke direkt nach Subscribe |
| | ← | `systems.localStats` | – | periodischer Push |
| Systemzustand | ← | `systems.full` / `systems.update` | – | wie bisher (von der Lib genutzt) |

> Die Library abonniert aktuell `systems.stats`. In FW 1.8.x lädt das Frontend Stats über
> `localStats.subscribe` + `systems.localStats*`.

## 2. Detail-Subscriptions (neu)

Die Einstellungs-Dialoge laden ihre Daten **nicht** aus `systems.full`, sondern abonnieren
beim Öffnen eine Detailansicht und kündigen sie beim Schließen wieder.

### 2.1 `state.subscribe` / `state.unsubscribe` – Encoder, Decoder, SRT

```jsonc
→ {"method":"state.subscribe","params":{"sysid":"<lokale sysid>","id":"<unit id>"}}
← {"result":{"method":"state.subscribe","id":"<unit id>","dataid":1,"name":"<unit name>",
             "sysName":"<system name>","response":true,"data":{...}}}
→ {"method":"state.unsubscribe","params":{"sysid":"<lokale sysid>","id":"<unit id>"}}
← {"result":{"method":"state.unsubscribe","sysid":"...","id":"...","response":true}}
```

- `sysid` ist immer das **lokale** System, auch wenn die Unit zu einem Remote-System gehört
  (z. B. `id:"X8A2222-E5"` über das lokale `X8A1111`).
- Unit-IDs: `<sysid>-E<n>` (XLink-Encoder), `<sysid>-D<n>` (XLink-Decoder),
  `<sysid>-srtE<n>` (SRT-Sender), `<sysid>-srtD<n>` (SRT-Receiver).

**`data` – gemeinsame Felder**

| Key | Typ | Bedeutung |
|---|---|---|
| `stateType` | number | **1** = XLink-Encoder, **2** = XLink-Decoder, **8** = SRT-Sender, **9** = SRT-Receiver |
| `nmosID` | object | `{device, video, audio, anc}` (UUIDs) |
| `mVideo`, `sdiDrivers`, `sdiDriversV2`, `running`, `xlinkRTT` | | Status |
| `values` | object | **aktuelle Konfiguration** (siehe unten) |
| `peers[]` | array | bekannte Remote-Systeme (wie `remote[]` in `systems.full`, gekürzt) |
| `ndifind[]` | array | gefundene NDI-Quellen |
| *Optionslisten* | `[{id,name,...}]` | Wertebereiche für die Dropdowns, siehe 2.2 |

### 2.2 Optionslisten (Enums) aus `state.subscribe`

| Key | Werte (`id`=Anzeige) | Encoder | Decoder | SRT |
|---|---|---|---|---|
| `vCard` | `0`=None, `10`=NDI, `12`=2110, `1`..`8`=SDI (1..8); Einträge können `disabled:true` sein, dazu Flags `av2110, ndi, hdmi, sdi, sdilevelA` | ✓ | ✓ | ✓ (ohne NDI/2110) |
| `av2110Net` | `none`, `eth6` (2110-fähige ETHs) | ✓ | ✓ | |
| `ethSoMark` | `auto`=Auto, `eth0`, `eth1`, `eth2`, `eth6` | ✓ | ✓ | |
| `vMode` / `vModeB` | `auto`=Auto sense, `ntsc`, `pal`, `hp50`/`hp59`/`hp60`=720p50/59.94/60, `23ps`=1080p23.98, `24ps`=1080p24, `Hp25`, `Hp29`, `Hp30`, `Hi50`/`Hi59`/`Hi60`=1080i, `Hp50`/`Hp59`/`Hp60`=1080p | ✓ | `vMode` | `vMode` |
| `vCodec` | `1`=H.264, `2`=H.265, `4`=VC-5 | ✓ | | |
| `vVC5q` | `{min:1, max:6}` | ✓ | | |
| `vColor` | `420`, `422` | ✓ | | |
| `vBit` | `8`=8 Bit, `10`=10 Bit | ✓ | | ✓ |
| `vNoS` | `0`=Off, `1`=Bars, `2`=Last Frame | ✓ | | |
| `vTBR` | `{vMaxBr:150, vMinBr:1}` (Mbps) | ✓ | | ✓ |
| `aMode` | Enc: `1`=Uncompressed, `2`=Compressed · SRT-RX: `0`=Auto, `1`=AAC, `2`=MP2, `3`=OPUS, `4`=AAC 2ch | ✓ | | ✓ (RX) |
| `aBit` | `16`, `32` | ✓ | | |
| `aCard` | `1`=SDI embedded | ✓ | | |
| `aCh` | Enc: `2`, `8`, `16` · SRT: `0`..`16` in 2er-Schritten | ✓ | | ✓ (TX) |
| `mpoint` | `none`=None | ✓ | | |
| `vBnoInType` | `Bars`, `Black` | | ✓ | |
| `vModeC` | `none`=No conversion, `ltbx`=Letterbox Downconversion, `amph`=Anamorphic Downconversion, `720c`=HD720→HD1080 | | ✓ | |
| `mode` | `1`=Listener, `2`=Caller | | | ✓ |
| `pbkeylen` | `16`=AES-128, `24`=AES-192, `32`=AES-256 | | | ✓ |

### 2.3 `sys.subscribe` / `sys.unsubscribe` – Systemdialoge

```jsonc
→ {"method":"sys.subscribe","params":{"id":"configSys"}}
← {"result":{"method":"sys.subscribe","id":"configSys","dataid":1,"sysid":"...","response":true,"data":{...}}}
→ {"method":"sys.unsubscribe","params":{"id":"configSys"}}
```

Beobachtete IDs:

| `id` | Dialog | `data` |
|---|---|---|
| `configSys` | System Config | `{sysid, name, type:"X8-R2", st2110, mtuTrunk, dns:{dnsStatic,dnsStaticIp1,dnsStaticIp2}, eth:[{id,name,st2110}], nmos:{nmos,nmosEth,nmosId,nmosDomain,nmosUseRegistry,nmosRegistryAddress,nmosRegistryVersion,nmosRegistrationPort,nmos*Label,nmos*Description}, ptp:{ptp,ptpEth,ptpDomainNumber,ptpDscp,ptpPriority1,ptpPriority2,ptpHybrid_e2e,ptpLogAnnounceInterval,ptpAnnounceReceiptTimeout,ptpSyncReceiptTimeout,ptpLogSyncInterval,ptpLogMinDelayReqInterval}, sysVer:{system,isDl,isDlId,getFrom,lastMd5Time,files[],peers[],img[]}}` |
| `users` | User Settings | `{users:[{id,system,enabled,global,beta,uuid,userId,firstName,lastName,info,type,role,timeFrom,timeTo,logonTime,logoffTime,logonIp}]}` |

> In `systems.full` liegt `configSys` **flach** vor; `sys.subscribe configSys` liefert es
> **gruppiert** (`dns`, `nmos`, `ptp`) plus `eth[]`, `sysVer` und `mtuTrunk`.

Für XLink Ports, Profile, Admin Proxy und die ETH-Dialoge wurde beim Öffnen **kein**
Request beobachtet – die Daten kommen dort aus `systems.full`.

### 2.4 Typunterschiede gegenüber `systems.full`

| Feld | `systems.full` | `state.subscribe` |
|---|---|---|
| `v2110NetPriPort`, `a2110NetPriPort` | **string** (`"5000"`) | **number** (`30000`) |
| `values.audio` | fehlt in `enc[].values`/`dec[].values` | vorhanden (bool) |
| `values.vHDR` | uneinheitlich string/number | number |

---

## 3. Dialoge & Felder

Alle Einheiten-Dialoge haben **keinen Speichern-Button**: Schalter und Dropdowns wirken
vermutlich sofort, Textfelder werden über einen „Change“-Schalter entsperrt.

### 3.1 Sender (XLink-Encoder, `stateType 1`)

Tabs: **General · Video · Audio · Source (› Video, › Audio bei 2110) · XLink**

| Tab | UI-Feld | `values`-Key |
|---|---|---|
| General | Name (+Change) | `name` |
| | Receiver | `receiver` (Unit-ID, z. B. `X8A1111-D1`) |
| | Auto Start | `autoStart` |
| | Bars standard | `vModeB` |
| | Start this time with Bars | – (einmalige Aktion) |
| | Reset Stats, START | Aktionen |
| Video | Codec | `vCodec` |
| | Bits | `vBit` |
| | Color | `vColor` |
| | Periodic intra refresh | `vPIntraOn` |
| | Interlaced encoding | `vIENC` |
| | CRF (Wert + ON / „CBR“ wenn aus) | `vCRF`, `vCRFOn` |
| | GOP (Wert + ON / „Auto“ wenn aus) | `vGOP`, `vGOPOn` |
| | FEC Level | `vFEC` (+ `vFecLDGM`) |
| | TBR (Mbps) | `vTBR` |
| | Reset Buffer | Aktion |
| Audio | Compression | `aMode` |
| | Bit depth | `aBit` |
| | Audio channels | `aCh` |
| | FEC | `aFEC` |
| | Bitrate per channel (Kbps) | `aTBR` |
| Source | Input | `vCard` |
| | Video standard (SDI) | `vMode` |
| | Audio (SDI) | `audio` |
| Source › Video (2110) | Enabled | `v2110Enabled` |
| | Lock Video (Auto=off) | `vModeLock` |
| | Video standard | `vMode` |
| | ETH | `v2110NetPri` |
| | Source IP | `v2110SDPSourceIp` |
| | Pri Enabled / Payload ID / IP / Port | `v2110NetPriEnabled`, `v2110RTPpayload`, `v2110NetPriIp`, `v2110NetPriPort` |
| | (Sec) | `v2110NetSec*`, `v2110SecSourceIp` |
| Source › Audio (2110) | Enabled, ETH, Source IP | `a2110Enabled`, `a2110NetPri`, `a2110SDPSourceIp` |
| | Pri Enabled / Payload ID / IP / Port | `a2110NetPriEnabled`, `a2110RTPpayload`, `a2110NetPriIp`, `a2110NetPriPort` |
| | Channels in SDP | `a2110SDPaCh` |
| | Packet Time (0.125 ms …) | `a2110PacketTime` |
| XLink | Packet ACK | `xAck` |
| | Resend | `xAckR` |
| | Max Resends | `xAckRmax` |
| | Resend Delay (ms) | `diffRtt` |
| | Max RTT (ms) | `maxRtt` |
| | Force Eth | `ethSoMark` |

Weitere Encoder-Keys ohne sichtbares UI-Feld im erkundeten Zustand: `vDeintI`, `vDeintPfs`,
`lowLatency`, `vCardSync`, `vCardConn`, `vPreset`, `vProfile`, `vULL`, `vVC5q`, `vCCB`,
`vThreads*`, `ndi*`, `xRTBR(On)`, `xAckRPct(Max)`, `oldRDP`, `rateControl`, `mtu(ON)`,
`mPointSender`, `a2110Config`, `v2110Config`, `a2110Silence`, `*SenderId`, `*SDPfile`.

### 3.2 Receiver (XLink-Decoder, `stateType 2`)

Tabs: **General · Signal Gen · Destination (› Video, › Audio bei 2110) · XLink**

| Tab | UI-Feld | `values`-Key |
|---|---|---|
| General | Name | `name` |
| | Sender | `sender` (Unit-ID) |
| | Auto Start | `autoStart` |
| | AV Sync | `vAVSOn` (+ `vAVDiff`) |
| | FPS Sync | `vFRCOn` |
| | Packet Buffer (ms, 10–…) | `pbuf` |
| | Allow Late Frames | `late` |
| | Reset Audio Buffer, Reset Stats, START, Restart XLink Tunnel | Aktionen |
| Signal Gen | Signal No In (ON) | `vBnoInOn` |
| | Time Delay (s) | `vBnoIn` |
| | Signal Type | `vBnoInType` |
| | Text size | `vBnoInH` |
| | Overlay X / Y pos | `vBnoInX`, `vBnoInY` |
| | Text (ON + Template; Variablen `%system% %sysname% %name% %format%`) | `vBnoInTextOn`, `vBnoInText` |
| | (Format/Name/SysName einblenden) | `vBnoInFormatOn`, `vBnoInNameOn`, `vBnoInSysNameOn` |
| Destination | Output | `vCard` |
| | Video standard (SDI) | `vMode` (+ `vModeA`) |
| | SDI Level A/B | `sdilevelA` |
| | 1080p Not PsF | `use1080pNotPsf` |
| Destination › Video (2110) | Enabled, ETH, Stop on No Signal | `v2110Enabled`, `v2110NetPri`, `v2110StopNoIn` |
| | Pri Enabled / Payload ID / IP / Port | `v2110NetPriEnabled`, `v2110RTPpayload`, `v2110NetPriIp`, `v2110NetPriPort` |
| Destination › Audio (2110) | Enabled, ETH, Stop on No Signal | `a2110Enabled`, `a2110NetPri`, `a2110StopNoIn` |
| | Pri Enabled / Payload ID / IP / Port | `a2110NetPriEnabled`, `a2110RTPpayload`, `a2110NetPriIp`, `a2110NetPriPort` |
| | Packet Time | `a2110PacketTime` |
| XLink | Force Eth | `ethSoMark` |

Weitere Decoder-Keys: `vFQOn`, `vBufferOn`, `vBuffer`, `vSDIBuffer`, `vModeC`,
`v2110DropOneVideo`, `vHDR`, `ndiAllowI`, `ndiName`, `ndiGroupOn`, `ndiGroup`, `maxRtt`, `oldRDP`.

### 3.3 SRT Sender (`stateType 8`)

Tabs: **General · Encoding · Source**

| UI-Feld | `values`-Key |
|---|---|
| Name, Auto Start | `name`, `autoStart` |
| Mode (Listener/Caller) | `mode` |
| Port / Address | `port`, `address` (+ `localPort`, `localPortOn`) |
| Latency | `latency` |
| Encryption (AES-128/192/256), Passphrase, Encrypt | `pbkeylen`, `passphrase`, `encrytion` *(sic)* |
| (Stream ID) | `streamid`, `streamidOn` |
| Bits | `vBit` |
| Periodic intra refresh | `vPIntraOn` |
| GOP | `vGOP`, `vGOPOn` |
| Audio Bitrate per channel | `aTBR` |
| Video Bitrate | `vTBR` |
| Input | `vCard` |
| Audio channels | `aCh` |

### 3.4 SRT Receiver (`stateType 9`)

Tabs: **General · Destination** – wie SRT Sender (`mode`, `address`, `port` als Source Port,
`localPort`/`localPortOn` als Destination Port, `latency`, `pbkeylen`, `passphrase`,
`encrytion`), plus `Output` = `vCard`, `Audio` = `audio`, Audio-Codec = `aMode`.

### 3.5 Netzwerk › Eth (Dialog „Settings ‹sys› – ethN“)

| UI-Feld | Key in `network.nets[]` |
|---|---|
| Type, MAC | read-only |
| Enabled | `enabled` |
| Admin Only ETH | `adminOnly` |
| Default External / Lan (NDI) / Backup External | `default`, `defaultLan` bzw. `ndi` (unklar), `backup` *(nur Nicht-Admin-ETHs)* |
| DHCP / Static | `dhcp` |
| IP, Gate, Mask, Dns 1/2 (per „Change“) | `ip`, `gate`, `mask`, `dns1`, `dns2` |
| WebAdmin | `admin` |
| WebAdmin – Only allow https connections | `adminSslOnly` *(neu, nur Admin-ETH)* |
| IGMP | `igmp` |
| NMOS | `nmos` *(neu)* |

Sidebar-Status pro ETH: Link (Up/Speed), Link Up seit, Internet, Gate Ping, Static, IP.

### 3.6 Netzwerk › XLink Trunk

„XLink Layer 2 Trunk“ – Liste (entspricht `l2s[]`) + „Add New“. Pro Trunk ist die MTU
überschreibbar, Default ist `mtuTrunk`.

### 3.7 System

- **General:** Cloud XLink Reg, Reg Info, Running Profile, System ID, Version, Power.
- **Stats:** PTP/NMOS Running, Video Channel License (gesamt / in use), Decoder Only License,
  System Uptime, CPU % / Temp, RX/TX pro ETH.
- **Settings:**
  - Name (+Change)
  - **XLink Ports Settings:** System-Port Dynamic/Static + Port; Data Port Range
    Dynamic/Static + Start/End; je „Update“; Neustart nötig.
    (`localSysport`, Peer-Struktur `ports:{sysPort,sysOn,portsOn,portsFrom,portsTo}`)
  - **Profile Settings:** Main (Profile-Liste; Edit Name, Reset settings, Swap to profile,
    Delete profile, Start/Stop), Import/new (Import-Text, „Copy eth/port settings from
    Active profile“, New Profile), Export (an Remote-System-ID, Export-Text).
  - **System Config** (`sys.subscribe configSys`):
    - PTPv2: ETH, Enabled, Domain Number, Hybrid e2e, Announce Interval, Announce Receipt
      Timeout, Min Delay Req Interval, DSCP. *(Priority1/2, Sync Interval und Sync Receipt
      Timeout sind im JSON, aber nicht in der UI.)*
    - NMOS: SysId, ETH (`nmosEth`), Enabled, Use Registry, Domain, Registry Port/Address,
      Label/Description-Templates (Variablen `%sysId% %sysName% %nodeLabel% %nodeDes%
      %name% %id% %id000% %id001%`).
    - DNS & MTU: Static DNS für alle ETHs (`dnsStatic`, `dnsStaticIp1/2`), System-MTU
      (`mtuTrunk`).
    - Version: Download from (Cloud), Check for updates, Versionsliste mit Download.
    - License: Import License.
    - Add System: Remote-System-ID → Send Request.
  - **User Settings** (`sys.subscribe users`): Create / Import / Export User(s);
    Tabelle Name, Role, Status, Expiry, Last Login, Actions.
  - **Admin Proxy:** 4 Slots (Port 81–84) → Remote-System; wird beim Neustart zurückgesetzt.

### 3.8 Video (Sidebar)

Tabs Senders / Receivers / NDI / SRT: Listen der Units mit Status, „Add New“, „On Dash“.

### 3.9 Remote Systems (Sidebar)

Configured / Discovered On Network. Pro System dieselbe Struktur wie lokal
(System/Video/Network) plus Verbindung: XLink P2P, P2P IP, Port, Connected since,
Manual IP Connect, System ID, Version, Power.

---

## Offene Punkte

- **Schreib-Requests** (Ändern von Werten, Start/Stop, Profile, Trunks …) sind noch nicht
  aufgezeichnet. Nächster Schritt: Änderungen gezielt in der UI auslösen und mitschneiden.
- Welche weiteren `sys.subscribe`-IDs existieren (Profile, Ports, Proxy, Version …)?
- Bedeutung einiger Keys ohne UI-Pendant (`vULL`, `vCCB`, `rateControl`, `oldRDP`, …).
