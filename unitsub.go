package xlinkclient

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
)

// configResult is the outcome of a state.subscribe request.
type configResult struct {
	id  UnitID
	raw json.RawMessage
	err error
}

// subscribeUnits keeps the configuration of the units in want subscribed
// until ctx is done. Each set received replaces the previous one: units no
// longer in it are unsubscribed, new ones subscribed once. The responses are
// handed to readLoop.
func (c *Client) subscribeUnits(ctx context.Context, want <-chan map[UnitID]bool, results chan<- configResult) {
	subscribed := map[UnitID]bool{}
	for {
		var ids map[UnitID]bool
		select {
		case <-ctx.Done():
			return
		case ids = <-want:
		}
		for _, id := range slices.Sorted(maps.Keys(subscribed)) {
			if ids[id] {
				continue
			}
			delete(subscribed, id)
			params := unitParams{SysID: c.SystemID(), ID: string(id)}
			if err := c.call(ctx, methodUnsubscribeUnit, params); err != nil && ctx.Err() == nil {
				c.logger.Debug("unsubscribing unit failed", slog.String("unit", string(id)), slog.Any("error", err))
			}
		}
		for _, id := range slices.Sorted(maps.Keys(ids)) {
			if subscribed[id] {
				continue
			}
			subscribed[id] = true
			raw, err := c.request(ctx, methodSubscribeUnit, unitParams{SysID: c.SystemID(), ID: string(id)})
			if ctx.Err() != nil {
				return
			}
			select {
			case results <- configResult{id: id, raw: raw, err: err}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// setLatest replaces the value waiting in ch, which must have a capacity of
// one and a single sender.
func setLatest[T any](ch chan T, v T) {
	select {
	case <-ch:
	default:
	}
	ch <- v
}
