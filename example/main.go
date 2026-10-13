// Command example connects to one or more VideoXLink systems and prints their
// state and statistics.
//
//	XLINK_PASSWORD=... go run ./example 10.0.0.1 10.0.0.2
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	xlinkclient "github.com/lukirs95/goxlinkclient/v5"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: example <address>...")
		os.Exit(2)
	}
	password := os.Getenv("XLINK_PASSWORD")
	logger := slog.Default()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// One pair of channels serves all systems.
	updates := make(chan xlinkclient.Update)
	stats := make(chan xlinkclient.StatsUpdate)

	var wg sync.WaitGroup
	for _, addr := range os.Args[1:] {
		client := xlinkclient.New(addr,
			xlinkclient.WithCredentials("admin", password),
			xlinkclient.WithLogger(logger),
			xlinkclient.WithUpdates(updates),
			xlinkclient.WithStats(stats),
		)
		wg.Add(1)
		go func() {
			defer wg.Done()
			run(ctx, client, logger)
		}()
	}

	// Each kind of update has its own handler.
	handlers := map[xlinkclient.UpdateKind]func(xlinkclient.Update){
		xlinkclient.UpdateSnapshot: func(u xlinkclient.Update) {
			sys := u.Snapshot
			fmt.Printf("%s (%s, %s): %d encoders, %d decoders\n",
				sys.Name, sys.ID, sys.Version, len(sys.Encoders), len(sys.Decoders))
		},
		xlinkclient.UpdateEncoder: func(u xlinkclient.Update) {
			if u.Removed {
				fmt.Printf("%s: encoder %s removed\n", u.System, u.ID)
				return
			}
			enc := u.Encoder
			fmt.Printf("%s: encoder %s %q running=%t video=%s receiver=%s\n",
				u.System, enc.ID, enc.Name, enc.Running, enc.VideoIn, enc.Receiver.ID)
		},
		xlinkclient.UpdateDecoder: func(u xlinkclient.Update) {
			if u.Removed {
				fmt.Printf("%s: decoder %s removed\n", u.System, u.ID)
				return
			}
			dec := u.Decoder
			fmt.Printf("%s: decoder %s %q running=%t video=%s sender=%s\n",
				u.System, dec.ID, dec.Name, dec.Running, dec.VideoOut, dec.Sender.ID)
		},
	}

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case u := <-updates:
			if h := handlers[u.Kind]; h != nil {
				h(u)
			}
		case s := <-stats:
			fmt.Printf("%s: CPU %d%% %d°C, PTP sync %t\n",
				s.Stats.System, s.Stats.Health.CPU, s.Stats.Health.CPUTemp, s.Stats.Health.PTPSync)
			for _, dec := range s.Stats.Decoders {
				if dec.Running {
					fmt.Printf("  %s %.2f fps, %.1f Mbps, sender RTT %v, %d frames dropped\n",
						dec.ID, dec.OutputFPS, dec.RX, dec.Sender.XLink.RTT, dec.Video.Dropped)
				}
			}
		}
	}
}

// run keeps the client connected until ctx is done, reconnecting after a
// short delay.
func run(ctx context.Context, client *xlinkclient.Client, logger *slog.Logger) {
	for {
		if err := client.Run(ctx); err != nil {
			logger.Error("connection failed", slog.String("addr", client.Addr()), slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
