# Go Client for communicating with Video XLink Devices

## Example

Just read out data and statistics like so

``` golang
package main

import (
	"context"
	"encoding/json"
	"fmt"

	xlinkclient "github.com/lukirs95/goxlinkclient"
)

func main() {
	stats := make(xlinkclient.StatsChan)
	updates := make(xlinkclient.UpdateChan)

	ctx := context.Background()

	go func() {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Context Done")
                return
			case update := <-updates:
				// read system updates here
			case stat := <-stats:
				// read statistics here
			}
		}
	}()

	client := xlinkclient.NewClient("10.199.128.90")
	client.Connect(ctx, updates, stats)
}
```