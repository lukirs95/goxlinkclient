package test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	xlinkclient "github.com/lukirs95/goxlinkclient"
)

var log *slog.Logger = slog.Default()

func main() {
	stats := make(xlinkclient.StatsChan)
	updates := make(xlinkclient.UpdateChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Context Done")
				return
			case update := <-updates:
				if upd, err := json.Marshal(update); err != nil {
					fmt.Println(err.Error())
				} else {
					fmt.Println(string(upd))
				}
			case stat := <-stats:
				fmt.Printf("System: %s\n", stat.SystemStats().Ident())
				for _, ethStat := range stat.EthStats() {
					fmt.Printf("Int: %s, RX: %f, TX: %f\n", ethStat.Ident(), ethStat.RX(), ethStat.TX())
				}
				for _, decStat := range stat.DecoderStats() {
					fmt.Printf("ID: %s, Drop: %d, FPS: %f, StatsTime: %d\n", decStat.Ident(), decStat.Dropped(), decStat.VideoOutFps(), decStat.StatsTime())
				}
				for _, encStat := range stat.EncoderStats() {
					fmt.Printf("ID: %s, FPS: %f, StatsTime: %d\n", encStat.Ident(), encStat.VideoInFps(), encStat.StatsTime())
				}
			}
		}
	}()

	client := xlinkclient.NewClient("10.199.128.90", xlinkclient.WithLogger(log))
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := client.Connect(ctx, updates, stats); err != nil {
			log.Error(err.Error())
		}
	}()

	time.Sleep(5 * time.Second)
	cancel()

	wg.Wait()
}
