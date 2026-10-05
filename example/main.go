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

	xlinkclient "github.com/lukirs95/goxlinkclient/v4"
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

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case u := <-updates:
			sys := u.System
			fmt.Printf("%s (%s, %s): %d encoders, %d decoders\n",
				sys.Name, sys.ID, sys.Version, len(sys.Encoders), len(sys.Decoders))
			for _, enc := range sys.Encoders {
				fmt.Printf("  %s %-20s running=%t video=%s receiver=%s\n",
					enc.ID, enc.Name, enc.Running, enc.VideoIn, enc.Receiver.ID)
			}
		case s := <-stats:
			if sys := s.Stats.SystemStats(); sys != nil {
				fmt.Printf("%s: CPU %d°C\n", s.Client.SystemID(), sys.CPUTemp())
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
