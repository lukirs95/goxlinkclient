# goxlinkclient

Go client for VideoXLink systems. There is no official API; the client speaks
the JSON-RPC protocol of the web frontend, which is documented in
[docs/ui-protocol.md](docs/ui-protocol.md).

Supported firmware: 1.8.4. **Firmware 1.7 is deprecated and not supported.**
Its state messages still decode, but unit statistics require
`localStats.subscribe`, which is only verified on 1.8.

```sh
go get github.com/lukirs95/goxlinkclient/v4
```

## Reading state

The client keeps the state of the system: it applies `systems.full` and the
`systems.update` deltas and delivers a complete `System` snapshot after every
change. The update and statistics channels can be shared by many clients; a
slow consumer receives the most recent snapshot of each system and never blocks
a connection.

```go
updates := make(chan xlinkclient.Update)

client := xlinkclient.New("10.0.0.1",
	xlinkclient.WithCredentials("admin", password),
	xlinkclient.WithUpdates(updates),
)
go client.Run(ctx) // blocks until ctx is done or the connection drops

for u := range updates {
	for _, enc := range u.System.Encoders {
		fmt.Println(enc.ID, enc.Name, enc.Running, enc.VideoIn.Present())
	}
}
```

`Run` returns when the connection ends; call it again to reconnect. See
[example/main.go](example/main.go) for several systems with reconnects.

## Statistics

With `WithStats(ch)` the client delivers a `Stats` value about every two
seconds. Unit and interface statistics come from `systems.localStats`; the
system health (CPU, temperatures, PTP sync, licenses) and the health of remote
systems come from `systems.stats`, which `systems.localStats` does not include.

```go
stats := make(chan xlinkclient.StatsUpdate)
client := xlinkclient.New(addr, xlinkclient.WithCredentials("admin", password),
	xlinkclient.WithStats(stats))

for s := range stats {
	for _, dec := range s.Stats.Decoders {
		fmt.Println(dec.ID, dec.OutputFPS, dec.XLink.RTT, dec.Receive.Health)
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
