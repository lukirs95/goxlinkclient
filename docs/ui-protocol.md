# VideoXLink Web-UI – Konfiguration & JSON-RPC (FW 1.8.4.6)

Reverse-Engineering-Protokoll des Web-Frontends (PrimeVue-App) eines X8-R2-Systems mit
Firmware **1.8.4.6**.

- Abschnitte 1–3: rein lesende Erkundung (Dialoge geöffnet, Tabs gewechselt).
- Abschnitt 4: Mitschnitte beim Umkonfigurieren eines Remote-Senders, eines lokalen
  Receivers und von SRT-Units, inkl. Start/Stop, Anlegen und Löschen.
- Abschnitt 5: Mitschnitte der System-, Netzwerk- (ETH inkl. IP) und Trunk-Einstellungen.

Was noch fehlt, steht unter [Offene Punkte](#offene-punkte).

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

### 1.1 `systems.update` – Delta-Format

`systems.full` liefert den kompletten Zustand, `systems.update` danach nur Änderungen
(etwa alle 5 s, auch ohne echte Änderung wegen `sysST`, `linkTime`, `lastSeenT`).

```jsonc
{"method":"systems.update","params":{"sysid":"<lokal>","dataid":2352,"data":{
  "local":{"dec":[{"id":"<sysid>-D6", ...geänderte Felder...}],
           "network":{"nets":[{"id":"eth0","linkTime":"…"}]}},
  "remote":[{"sysid":"<remote>","lastSeenT":"…"}]}}}
```

- Nur geänderte Felder werden geschickt, auch verschachtelt (`values`, `receiver.values`).
- **Arrays sind per Schlüssel adressiert, nicht per Index:** `id` bei `enc`, `dec`,
  `network.nets` (und vermutlich `srt`, `ndi`, `l2s`), `sysid` bei `remote`.
- **Neues Element:** kommt vollständig, inklusive `type` (= `stateType`) und `config:true`.
- **Gelöschtes Element:** `{"id":"…","delete":true}`. Bei Decodern kommt davor
  `{"id":"…","enabled":false}`.
- IDs werden nach dem Löschen wiederverwendet (`E6`/`D6` erneut vergeben).
- `dataid` zählt pro Nachricht hoch, damit lassen sich Lücken erkennen.
- **SRT und NDI:** `local.srt[]` bzw. `local.ndi[]` enthalten Sender **und** Empfänger
  gemischt. Unterscheidbar sind sie über `type` (8/9 bzw. 3/4) oder das ID-Präfix
  (`srtE`/`srtD`, `NdiE`/`NdiD`); nur das Präfix steht auch in Teil-Deltas. In
  `systems.full` heißen die SRT-Felder anders als bei `config`/`state.subscribe`:
  `srtMode`, `srtAddress`, `srtPort` (String), `srtLocalPort` (String), `srtEncrytion`
  (sic); dazu kommt `runError` auf Element-Ebene. NDI-Empfänger melden `ndiSource`,
  `ndiSourceName` und `ndiSourceCon`.
- `vTBR` (Bitrate) steht in `systems.full` als Gleitkommazahl (`25.0`), beim Schreiben als
  Ganzzahl.
- Innerhalb **einer** Nachricht sind Typen uneinheitlich: In `enc[].values` ist
  `v2110NetPriPort` eine Zahl, in `enc[].receiver.values` ein String. Dasselbe gilt für
  `vHDR` und `vBitInit`.

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
  `<sysid>-NdiE<n>` (NDI-Sender), `<sysid>-NdiD<n>` (NDI-Receiver), `<sysid>-srtE<n>` (SRT-Sender),
  `<sysid>-srtD<n>` (SRT-Receiver).

**`data` – gemeinsame Felder**

| Key | Typ | Bedeutung |
|---|---|---|
| `stateType` | number | **1** = XLink-Encoder, **2** = XLink-Decoder, **3** = NDI-Sender (SDI→NDI), **4** = NDI-Receiver (NDI→SDI), **8** = SRT-Sender, **9** = SRT-Receiver – identisch mit `type` bei `newVideo` |
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
| | Reset Stats → `resetVstat`, START → `start` | Aktionen (siehe 4.3) |
| Video | Codec | `vCodec` |
| | Bits | `vBit` |
| | Color | `vColor` |
| | Periodic intra refresh | `vPIntraOn` |
| | Interlaced encoding | `vIENC` |
| | CRF (Wert + ON / „CBR“ wenn aus) | `vCRF`, `vCRFOn` |
| | GOP (Wert + ON / „Auto“ wenn aus) | `vGOP`, `vGOPOn` |
| | FEC Level | `vFEC` (+ `vFecLDGM`) |
| | TBR (Mbps) | `vTBR` |
| | Reset Buffer → `resetSSRC` | Aktion (siehe 4.3) |
| Audio | Compression | `aMode` |
| | Bit depth | `aBit` |
| | Audio channels | `aCh` |
| | FEC | `aFEC` |
| | Bitrate per channel (Kbps) | `aTBR` |
| Source | Input | `vCard` |
| | Video standard (SDI) | `vMode` |
| | Audio (SDI) | `audio` |
| Source › Video (2110) | Enabled | `video` *(beobachtet; nicht `v2110Enabled`)* |
| | Lock Video (Auto=off) | `vModeLock` |
| | Video standard | `vMode` |
| | ETH | `v2110NetPri` |
| | Source IP | `v2110SDPSourceIp` |
| | Pri Enabled / Payload ID / IP / Port | `v2110NetPriEnabled`, `v2110RTPpayload`, `v2110NetPriIp`, `v2110NetPriPort` |
| | (Sec) | `v2110NetSec*`, `v2110SecSourceIp` |
| Source › Audio (2110) | Enabled, ETH, Source IP | Enabled nicht mitgeschnitten (vermutl. `audio`), `a2110NetPri`, `a2110SDPSourceIp` |
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
| | Reset Audio Buffer → `flushAudio`, Reset Stats → `resetVstat`, START → `start`, Restart XLink Tunnel (nicht mitgeschnitten) | Aktionen (siehe 4.3) |
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
| Destination › Video (2110) | Enabled, ETH, Stop on No Signal | `v2110NetPriEnabled` *(wie Pri Enabled)*, `v2110NetPri`, `v2110StopNoIn` |
| | Pri Enabled / Payload ID / IP / Port | `v2110NetPriEnabled`, `v2110RTPpayload`, `v2110NetPriIp`, `v2110NetPriPort` |
| Destination › Audio (2110) | Enabled, ETH, Stop on No Signal | `a2110NetPriEnabled` *(wie Pri Enabled)*, `a2110NetPri`, `a2110StopNoIn` |
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
| Listener: Port | `localPort` |
| Caller: Address, Port, Source Port (+Schalter) | `address`, `port`, `localPort`, `localPortOn` |
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
| Default External / Lan (NDI) / Backup External | `default`, `defaultLan` *(per Mitschnitt bestätigt)*, `backup` *(nur Nicht-Admin-ETHs)* |
| DHCP / Static | `dhcp` |
| IP, Gate, Mask, Dns 1/2 (per „Change“) | `ip`, `gate`, `mask`, `dns1`, `dns2` – zusammen mit `dhcp:false` in einem `configEth` |
| WebAdmin | `admin` |
| WebAdmin – Only allow https connections | `adminSslOnly` *(neu, nur Admin-ETH)* |
| IGMP | `igmp` |
| NMOS | `nmos` *(neu)* |

Sidebar-Status pro ETH: Link (Up/Speed), Link Up seit, Internet, Gate Ping, Static, IP.

**SMPTE 2110 – Hardware-Einschränkung**

| Hardware | 2110-fähige ETHs |
|---|---|
| X8 (z. B. `configSys.type = "X8-R2"`) | nur **eth6** und **eth7** |
| X4 | nur **eth2** und **eth3** |

Das Gerät meldet das selbst: `sys.subscribe configSys` liefert `eth:[{id, name, st2110}]`, und
`state.subscribe` die Optionsliste `av2110Net` (nur `none` + 2110-fähige ETHs). Beide Listen
enthalten nur **aktivierte** ETHs. Ein deaktiviertes eth7 fehlt also, obwohl es 2110 könnte.
Eine Library sollte `v2110NetPri`/`a2110NetPri`, `nmosEth` und `ptpEth` gegen diese Listen
prüfen statt ETH-Namen fest zu verdrahten.

### 3.6 Netzwerk › XLink Trunk

„XLink Layer 2 Trunk“ – Liste (entspricht `l2s[]`) + „Add New“. Pro Trunk ist die MTU
überschreibbar, Default ist `mtuTrunk`. Die Schreib-Requests stehen in [5.1](#51-xlink-layer-2-trunks).

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

## 4. Schreib-Requests (Encoder / Decoder)

Mitgeschnitten an einem Remote-Encoder (`<remote>-E5`, angesprochen über das lokale System)
und einem lokalen Decoder (`<local>-D1`).

### 4.1 `config` – eine Einstellung ändern

```jsonc
→ {"method":"config","params":{"sysid":"<lokale sysid>","id":"<unit id>","values":{"<key>":<wert>}}}
← {"result":{"method":"config","id":"<unit id>","response":true,"values":true}}
← {"method":"state.update","params":{"id":"<unit id>","dataid":<n>,"data":{"values":{...}}}}   // Push an alle Abonnenten
```

- Das Frontend schickt **immer genau einen Key pro Request**, und zwar sofort bei jeder
  Änderung. Slider (z. B. TBR) erzeugen beim Ziehen einen Request pro Zwischenwert.
- Wie bei `state.subscribe` ist `sysid` das **lokale** System, auch bei Remote-Units.
- `state.update` kommt nur, solange die Unit per `state.subscribe` abonniert ist. Das Delta
  enthält geänderte `values` und ggf. geänderte Optionslisten/Limits (siehe 4.4).
- Die Library nutzt `config` bereits (`EnableVideo`/`DisableVideo` mit `v2110NetPriEnabled`).
  Das passt zum Decoder, beim Encoder schaltet die UI 2110-Video aber über `video`.

**Typen, wie sie die UI sendet**

| Typ | Keys |
|---|---|
| bool | `autoStart`, `vPIntraOn`, `vIENC`, `vCRFOn`, `vGOPOn`, `aFEC`, `video`, `v2110NetPriEnabled`, `a2110NetPriEnabled`, `xAck`, `xAckR`, `vAVSOn`, `vFRCOn`, `late`, `vBnoInOn`, `vBnoInTextOn`, `v2110StopNoIn`, `a2110StopNoIn` |
| number | `vCodec`, `vColor`, `vBit`, `vCRF`, `vGOP`, `vFEC` (0–3), `vTBR`, `aMode`, `aCh`, `aTBR`, `a2110PacketTime`, `xAckRmax`, `diffRtt`, `maxRtt`, `pbuf`, `vBnoIn`, `vBnoInH`, `vBnoInX`, `vBnoInY` |
| string (Enum) | `vModeB`, `vModeLock`, `vMode`, `v2110NetPri`, `a2110NetPri`, `ethSoMark`, `vBnoInType`, `vCard` (`"12"`, `"3"` …), `name` |
| **string (numerischer Inhalt!)** | `v2110RTPpayload`, `a2110RTPpayload`, `v2110NetPriPort`, `a2110NetPriPort`, `a2110SDPaCh`, sowie IPs: `v2110SDPSourceIp`, `a2110SDPSourceIp`, `v2110NetPriIp`, `a2110NetPriIp` |

> Textfelder (Payload ID, Port, Channels in SDP) gehen als **String** raus, obwohl
> `state.subscribe` sie als Zahl liefert. Das erklärt die String-Ports in `systems.full`.
> Eine Library sollte beim Lesen beides akzeptieren.

### 4.2 Fehler

```jsonc
← {"error":{"code":-32603,"message":"Internal error","data":{"vModeLock":"Video Mode auto Not supported for Card 12"}}}
← {"method":"state.update","params":{"id":"...","data":{"values":{"vModeLock":"auto"},"error":{"vModeLock":"Video Mode auto Not supported for Card 12"}}}}
```

Bei `config` ist `error.data` ein Objekt `{<key>: <meldung>}`, bei Aktionen ein String
(`"video not running"`). Fehler werden zusätzlich als `state.update` mit `data.error` gepusht.

### 4.3 Aktionen

Alle mit `params: {sysid:"<lokale sysid>", id:"<unit id>"}`, Antwort `{method, id, response:true}`.

| Methode | UI | Unit | Bemerkung |
|---|---|---|---|
| `start` | START | Enc/Dec | danach `state.update` mit `data.running:true` (bereits in der Lib) |
| `stop` | STOP | Enc/Dec | (bereits in der Lib) |
| `resetVstat` | Reset Stats | Enc/Dec | |
| `resetSSRC` | Video › Reset Buffer | Enc | Fehler `"video not running"`, wenn gestoppt |
| `flushAudio` | General › Reset Audio Buffer | Dec | Fehler `"video not running"`, wenn gestoppt |
| `deleteVideo` | Unit löschen | Enc/Dec/SRT | Antwort `{method, sysid, response:true}`; funktioniert für `-E`, `-D`, `-srtE`, `-srtD` |
| `newVideo` | Video › Senders/Receivers/NDI/SRT › Add New | – | **ohne `id`**: `params:{sysid, values:{type}}`, `type` = `stateType` (1 Enc, 2 Dec, 3 NDI-Sender, 4 NDI-Receiver, 8 SRT-Sender, 9 SRT-Receiver – alle per Mitschnitt bestätigt). Antwort `{method, sysid, response:true}` **ohne ID der neuen Unit** – die kommt nur über `systems.update` |

Noch nicht mitgeschnitten: „Restart XLink Tunnel“, „Start this time with Bars“, „Add New“
für SRT und NDI.

### 4.4 Abhängigkeiten / Nebenwirkungen

| Änderung | Effekt im `state.update` |
|---|---|
| `vCodec` → 2 (H.265) | `values.vTBR` wird auf 10 begrenzt, Limits `vTBR:{vMaxBr:10}` |
| `v2110NetPri`/`a2110NetPri` → `"none"` | `*NetPriEnabled` wird `false` |
| `pbuf` = n | `maxRtt` = n + 100 |
| `vCard` (Decoder) | `vModeA` ändert sich, Optionsliste `vCard` wird neu geschickt |
| `vModeLock = "auto"` bei `vCard "12"` (2110) | Fehler, nicht unterstützt |

### 4.5 SRT Sender / Receiver

SRT-Units nutzen dasselbe `config` (ein Key pro Request, `state.update`-Push). Beobachtete
Keys und Typen:

| Typ | SRT Sender (`-srtE<n>`) | SRT Receiver (`-srtD<n>`) |
|---|---|---|
| bool | `autoStart`, `encrytion` *(sic)*, `localPortOn`, `vPIntraOn`, `vGOPOn` | `autoStart`, `encrytion`, `localPortOn`, `audio` |
| number | `mode` (1 Listener / 2 Caller), `pbkeylen` (16/24/32), `vBit`, `vGOP`, `vTBR`, `aTBR`, `aCh` (0–16) | `mode`, `pbkeylen` |
| string | `name`, `address`, `vCard`, `vMode`, `passphrase` (Klartext!) | `name`, `address`, `vCard` |
| **string (numerisch)** | `localPort`, `port`, `latency` | `localPort`, `port`, `latency` |

- `vCard` ändern pusht jedes Mal die Optionsliste `vCard` neu.
- Die Passphrase geht im Klartext über den (unverschlüsselten) WebSocket.
- Beim SRT Receiver wurde `aMode` (Audio-Codec) nicht geändert.

### 4.6 NDI Sender / Receiver

Angelegt mit `newVideo type 3/4`, konfiguriert per `config`, gelöscht per `deleteVideo`.
Start/Stop wurde nicht mitgeschnitten (vermutlich `start`/`stop` wie bei Enc/Dec).

**NDI-Sender** (`-NdiE<n>`, `stateType 3`, SDI rein → NDI raus)

| Key | Typ (gesendet) | Bedeutung |
|---|---|---|
| `name`, `autoStart` | string, bool | |
| `vCard` | string | SDI-Eingang (`"0"` None, `"1"`..`"8"`) |
| `vMode`, `vModeB` | string | Video standard / Bars standard |
| `aCh` | number | Audio-Kanäle (2/8/16) |
| *(nur gelesen)* | | `ndiName`, `ndiGroupOn`, `ndiGroup`, `ndiAllowI`, `aMode`, `aSync`, `vBit`, `use1080pNotPsf`, `vModeC`, `mtu` (9000) |

**NDI-Receiver** (`-NdiD<n>`, `stateType 4`, NDI rein → SDI raus)

| Key | Typ (gesendet) | Bedeutung |
|---|---|---|
| `name`, `autoStart` | string, bool | |
| `vAVSOn`, `vFRCOn` | bool | AV Sync, FPS Sync |
| `ndiT` | bool | |
| `vSDIBuffer` | bool | |
| `aCh` | number | |
| `vCard`, `vMode` | string | SDI-Ausgang, Video standard (`vMode` pusht `vModeA` mit) |
| `sdilevelA` | **string `"true"`/`"false"`** | SDI Level A/B – gelesen als bool! |
| `use1080pNotPsf` | bool | |
| `vResize` | bool | |
| *(nur gelesen)* | | `ndiSource`, `ndiSourceName`, `ndiGroupsOn`, `ndiGroups`, `ndiAllowI`, `vAVDiff`, `vFQOn`, `vModeC`, `mtu` |

Die NDI-Quellenauswahl (`ndiSource`) wurde nicht gesetzt (`ndifind` war leer).

### 4.7 Im Mitschnitt nicht vorgekommen

Encoder: `name`, `receiver`, `vNoS`, Source-Input (`vCard`), Audio-Enabled, Sec-Netz (`*NetSec*`).
Decoder: `sender`, `vBnoInText`, `vBnoInFormatOn`/`NameOn`/`SysNameOn`, `sdilevelA`,
`use1080pNotPsf`, SDI-`vMode`.

---

## 5. Schreib-Requests (System)

Systemeinstellungen haben **eigene Methoden** statt `config`. Alle haben die Form
`params: {sysid:"<lokale sysid>", values:{...}}` (ohne `id`), Antwort
`{method, sysid|id, response:true}`. Pro Request wird ein Key bzw. eine zusammengehörige
Gruppe geschickt.

| Methode | UI | `values` | Push |
|---|---|---|---|
| `configSysName` | Settings › Name | `{name}` | `systems.update` |
| `configSysPorts` | XLink Ports › System | `{sysOn:true, sysPort:"10501"}` · Dynamic: `{sysOn:false}` | `systems.update` |
| `configSysPorts` | XLink Ports › Data Port Range | `{portsOn:true, portsFrom:"10502", portsTo:"10539"}` · Dynamic: `{portsOn:false}` | `systems.update` |
| `set2110` | System Config › PTPv2 | `{ptp}`, `{ptpEth}`, `{ptpDomainNumber}`, `{ptpHybrid_e2e}`, `{ptpLogAnnounceInterval}`, `{ptpAnnounceReceiptTimeout}`, `{ptpLogMinDelayReqInterval}`, `{ptpDscp}` | `sys.update configSys` |
| `set2110` | System Config › NMOS | `{nmos}`, `{nmosEth}` (weitere NMOS-Keys vermutlich gleich) | `sys.update configSys` |
| `dnsSys` | System Config › DNS & MTU | `{dnsStatic}`, `{dnsStaticIp1}`, `{dnsStaticIp2}` | `sys.update configSys` |
| `setSystemMTU` | System Config › DNS & MTU | `{mtuTrunk}` (number) | `sys.update configSys` |
| `manAddPeer` | System Config › Add System | `{systemId:"<remote sysid>"}` | – |
| `configEth` | Network › Eth › Settings | `{eth:"eth3", <key>:<wert>}` – ein Key pro Request: `enabled`, `default` (Default External), `defaultLan` (Lan (NDI)), `backup` (Backup External), `admin` (WebAdmin), `adminSslOnly`, `igmp`, `nmos` (alle bool). **IP-Konfiguration** als ein Request: statisch `{eth, dhcp:false, ip, mask, gate, dns1, dns2}` (alles Strings, z. B. `mask:"255.255.0.0"`), DHCP nur `{eth, dhcp:true}` | vermutl. `systems.update` (`network.nets[]`) |
| `manIpPeer` | Remote System › Manual IP Connect | `{peer:"<remote sysid>", manIp:"<ip>", manIpSec:"<ip>", manPort:"10501", manAutCon:false}` – immer der komplette Satz; Löschen = `manIp/manIpSec/manPort:""`, `manAutCon:false` | vermutl. `systems.update` (Peer-Felder `manIp`, `manIpSec`, `manPort`, `manAutCon`) |

**`sys.update`-Push** (solange `sys.subscribe configSys` aktiv ist):

```jsonc
← {"method":"sys.update","params":{"sysid":"local","id":"configSys","dataid":<n>,
    "data":{"ptp":{"ptpDomainNumber":100}, "sysVer":{...}}}}
```

Das Delta ist wie bei `sys.subscribe` gruppiert (`ptp`, `nmos`, `dns`, `mtuTrunk`) und
enthält jedes Mal zusätzlich `sysVer` (u. a. die Peer-Liste).

### 5.1 XLink Layer 2 Trunks

Trunk-IDs: `<sysid>-L2S<n>`. Ein Trunk verbindet ETHs mehrerer Systeme; dafür muss auf
**jedem** beteiligten System ein Trunk existieren, der Remote-Trunk wird dann zum lokalen
hinzugefügt.

| Methode | UI | `params` | Bemerkung |
|---|---|---|---|
| `newL2S` | Network › XLink Trunk › Add New | `{sysid:"<Zielsystem>"}` | **`sysid` = System, auf dem der Trunk entsteht** – auch ein Remote-System. Antwort `{method, sysid, id:"local", response:true}` **ohne Trunk-ID** (kommt über `systems.update`) |
| `configL2S` | Trunk-Einstellungen | `{sysid?, id:"<trunk id>", values:{<key>:<wert>}}` | ein Key pro Request; die UI lässt `sysid` oft weg (nur bei `master`/`delPeer` gesetzt) – `id` reicht offenbar |
| `startL2S` / `stopL2S` | Start/Stop | `{sysid, id}` | |
| `deleteL2S` | Trunk löschen | `{sysid:"<Besitzer-System>", id}` | |

**`configL2S`-Keys**

| Key | Typ | Bedeutung |
|---|---|---|
| `eth` | string | ETH, auf dem der Trunk liegt (z. B. `"eth3"`) |
| `master` | bool | Master-Seite des Trunks |
| `l2mtuOn`, `l2mtu` | bool, number | eigene MTU statt `mtuTrunk` (z. B. 1450) |
| `autoStart` | bool | |
| `encryption` | bool | (hier korrekt geschrieben, anders als SRT `encrytion`) |
| `multicast`, `rmcast` | bool | |
| `addPeer` | string | Remote-Trunk-ID hinzufügen, z. B. `"<remote>-L2S1"` |
| `delPeer` | string | Remote-Trunk-ID entfernen |

> Im alten Dump (`example/systemFull.go`) hat `l2s[].values` die Keys `autoStart`,
> `encryption`, `eth`, `master`, `masterId`, `multicast`, `rmcast` sowie `l2s[].members`.
> Neu sind `l2mtuOn`/`l2mtu`. `addPeer`/`delPeer` sind reine Schreib-Kommandos.

Beobachteter Ablauf: `newL2S` lokal → `configL2S eth/master/…` → `newL2S` auf dem Remote →
`configL2S` Remote `master:false`, `eth` → lokal `addPeer:"<remote trunk>"` → `startL2S` →
`stopL2S` → `delPeer` → `deleteL2S` (lokal, dann Remote).

Nebenbei: Vor dem Anlegen wurde eth3 per `configEth enabled:true` aktiviert. Das Feld
`backup:false` wurde gesendet und eth3 danach wieder deaktiviert.

### 5.2 Sonstiges

**`configEth` mit `response:false`:** `{eth:"eth3", defaultLan:false}` wurde mit
`response:false` abgelehnt, ohne JSON-RPC-Error (kurz zuvor war `defaultLan` auf eth2 gesetzt
worden). Clients müssen also neben `error` auch `result.response` prüfen. Die Library tut das
für `start`/`stop`/`config` bereits.

**Typen:** Wie bei `config` gehen Textfelder als **String** raus (`ptpDomainNumber:"100"`,
`ptpDscp:"47"`, `ptpLogAnnounceInterval:"-2"`, `sysPort:"10502"`). `sys.update` meldet sie als
Zahl zurück. Bei `configSysPorts` bleibt ein nicht bearbeitetes Feld eine Zahl
(`portsTo:10539`) und wird erst nach dem Bearbeiten zum String – die Typen sind also
**gemischt**. `setSystemMTU` schickt eine Zahl.

Ohne Request blieben in diesem Durchlauf: Profile Settings, User Settings, Admin Proxy,
Version und License. Entweder wurde dort nichts geändert, oder diese Dialoge laufen über
HTTP (vor diesem Durchlauf nicht mitgeschnitten).

---

## Offene Punkte

- NDI: Quellenauswahl (`ndiSource`), NDI-Name/Gruppen; Start/Stop für NDI/SRT (vermutlich wie Enc/Dec).
- System: Profile, User, Admin Proxy, Version/Update, License, weitere NMOS-Felder
  (Domain, Registry, Labels) sowie PTP-Werte ohne UI.
- Die in 4.7 genannten Encoder-/Decoder-Felder fehlen noch.
- Welche weiteren `sys.subscribe`-IDs existieren (Profile, Ports, Proxy, Version …)?
- Bedeutung einiger Keys ohne UI-Pendant (`vULL`, `vCCB`, `rateControl`, `oldRDP`, …).
