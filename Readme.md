# goxlinkclient

Go client for VideoXLink systems. There is no official API; the client speaks
the JSON-RPC protocol of the web frontend, which is documented in
[docs/ui-protocol.md](docs/ui-protocol.md). Tested against firmware 1.7.2 and
1.8.4.

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
Further requests: `Start`, `Stop`, `ResetStats`, `CreateUnit`, `DeleteUnit`,
`ConfigureInterface`, `ConfigurePTP`, `ConfigureNMOS`, `ConfigureDNS`,
`ConfigureTrunk`, `CreateTrunk`, `SetManualIP` and more.

The SMPTE ST 2110 capable interfaces depend on the hardware (X8: eth6/eth7, X4:
eth2/eth3); the device itself reports which of its enabled interfaces support
2110.
