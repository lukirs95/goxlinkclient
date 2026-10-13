# goxlinkclient

Go client for VideoXLink systems. There is no official API; the client speaks
the JSON-RPC protocol of the web frontend, which is documented in
[docs/ui-protocol.md](docs/ui-protocol.md).

Supported firmware: 1.8.4. **Firmware 1.7 is deprecated and not supported.**
Its state messages still decode, but unit statistics require
`localStats.subscribe`, which is only verified on 1.8.

```sh
go get github.com/lukirs95/goxlinkclient/v5
```

## Reading state

The client keeps the state of the system: it applies `systems.full` and the
`systems.update` deltas. With `WithUpdates(ch)` it delivers an
`UpdateSnapshot` with the complete `System` whenever a connection is opened,
afterwards one `Update` per element that was added, changed or removed: the
system's own fields (`UpdateSystem`), a unit, an interface, a trunk or a
remote system. An update always carries the complete element; `Kind` says
which field holds it, so a consumer can dispatch to specific handlers:

```go
updates := make(chan xlinkclient.Update)

client := xlinkclient.New("10.0.0.1",
	xlinkclient.WithCredentials("admin", password),
	xlinkclient.WithUpdates(updates),
)
go client.Run(ctx) // blocks until ctx is done or the connection drops

handlers := map[xlinkclient.UpdateKind]func(xlinkclient.Update){
	xlinkclient.UpdateSnapshot: func(u xlinkclient.Update) { /* u.Snapshot */ },
	xlinkclient.UpdateEncoder: func(u xlinkclient.Update) {
		if u.Removed {
			fmt.Println("encoder removed", u.ID)
			return
		}
		fmt.Println(u.Encoder.ID, u.Encoder.Name, u.Encoder.Running)
	},
}
for u := range updates {
	if h := handlers[u.Kind]; h != nil {
		h(u)
	}
}
```

Changes are detected while a message is applied: only the elements it
addresses are compared, and values the device repeats without change (start
and link times in every heartbeat) produce no update. The update channel and
the statistics channel can be shared by many clients. A slow consumer never
blocks a connection: a pending update is replaced by a newer one of the same
element, and a snapshot replaces the pending updates of its system.
`Snapshot` returns the current state at any time.

If the device's messages were lost (judged by their sequence numbers), `Run`
returns `ErrUpdatesLost`; the next `Run` starts with a new snapshot.

The configuration of a unit, i.e. what its settings dialog in the web UI
shows, is not part of `systems.full`. The client subscribes it with
`state.subscribe` for every local unit and for the counterparts of the local
XLink units and follows the `state.update` deltas. Loading and changing a
configuration delivers the complete unit; `Config` is nil until it has been
loaded:

```go
if enc := u.Encoder; enc != nil && enc.Config != nil {
	fmt.Println(enc.ID, enc.Config.Bitrate, enc.Config.GOP, enc.Config.MaxRTT)
	if enc.Receiver.Config != nil {
		fmt.Println(enc.Receiver.ID, enc.Receiver.Config.PacketBuffer)
	}
}
```

New units and new links are subscribed as they appear, deleted ones
unsubscribed. The fields mirror the settings of the `Configure` methods.

`Run` returns when the connection ends; call it again to reconnect. See
[example/main.go](example/main.go) for several systems with reconnects.

## Errors

A request rejected by the device returns a `*DeviceError` with the JSON-RPC
code and message and the device's explanation:

```go
var devErr *xlinkclient.DeviceError
if errors.As(err, &devErr) {
	fmt.Println(devErr.Detail) // e.g. "video not running"
	fmt.Println(devErr.Fields) // rejected config keys, e.g. map[vModeLock:...]
}
```

Requests while the client is not connected return `ErrNotConnected`.

## Statistics

With `WithStats(ch)` the client delivers a `Stats` value about every two
seconds. It contains the values the web UI shows. Unit and interface
statistics come from `systems.localStats`, completed with the values only
`systems.stats` reports: the ARQ details and audio levels of encoders, the
frame and packet counts of decoders, the counterparts' uptimes, SRT and NDI units, and
the health of the local and remote systems (CPU, temperature, PTP, licenses).

```go
stats := make(chan xlinkclient.StatsUpdate)
client := xlinkclient.New(addr, xlinkclient.WithCredentials("admin", password),
	xlinkclient.WithStats(stats))

for s := range stats {
	for _, dec := range s.Stats.Decoders {
		fmt.Println(dec.ID, dec.OutputFPS, dec.Video.Dropped, dec.Sender.XLink.RTT)
	}
}
```

The statistics history the web UI loads (`systems.localStatsHistory`) is not
used.

## Changing settings

Settings are typed per unit kind and sent in one request:

```go
err := client.ConfigureEncoder(ctx, "X8A1111-E1",
	xlinkclient.EncoderBitrate(25),
	xlinkclient.EncoderCodec(xlinkclient.VideoCodecH265),
	xlinkclient.EncoderVideo2110(xlinkclient.Stream{
		Interface: "eth6", Enabled: true, Address: "239.0.0.1", Port: 30000,
	}),
)
```

Units of remote systems are configured through the local system the same way.
Links between units are always made on the decoder, also for a decoder on a
remote system:

```go
// Let the remote decoder receive from the local encoder.
err := client.ConfigureDecoder(ctx, "X8A2222-D2", xlinkclient.DecoderSender("X8A1111-E1"))
```

Further requests: `Start`, `Stop`, `ResetStats`, `CreateUnit`, `DeleteUnit`,
`ConfigureInterface`, `ConfigurePTP`, `ConfigureNMOS`, `ConfigureDNS`,
`ConfigureTrunk`, `CreateTrunk`, `SetManualIP` and more.

## Integration tests

`integration_test.go` runs against a real system and **creates, changes and
deletes units** on it. It never touches eth0 and switches off outgoing 2110
streams of a peer decoder only temporarily, restoring them afterwards.
Credentials are read from the environment or a git-ignored `.env` file:

```sh
XLINK_ADDR=host XLINK_PEER=X8A... go test -tags integration -run Device -v -count=1 .
```

## Notes

The SMPTE ST 2110 capable interfaces depend on the hardware (X8: eth6/eth7, X4:
eth2/eth3); the device itself reports which of its enabled interfaces support
2110.
