package xlinkclient

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"time"
)

// configResult is the response to a state.subscribe request.
type configResult struct {
	id  UnitID
	raw json.RawMessage
}

// Retry delays for a failed state.subscribe. Requests for units of remote
// systems fail now and then, e.g. with "Not Connected" while the system is
// connected.
const (
	firstSubscribeRetry = 2 * time.Second
	maxSubscribeRetry   = time.Minute
)

// subscribeUnits keeps the configuration of the units in want subscribed
// until ctx is done. Each set received replaces the previous one: units no
// longer in it are unsubscribed, new ones subscribed. A failed subscription is
// retried with growing delays while the unit is wanted. The responses are
// handed to readLoop.
func (c *Client) subscribeUnits(ctx context.Context, want <-chan map[UnitID]bool, results chan<- configResult) {
	type retry struct {
		at    time.Time
		delay time.Duration
	}
	var (
		ids        map[UnitID]bool
		subscribed = map[UnitID]bool{}
		retries    = map[UnitID]retry{}
		timer      *time.Timer
		wake       <-chan time.Time
	)
	for {
		select {
		case <-ctx.Done():
			return
		case ids = <-want:
		case <-wake:
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
		for id := range retries {
			if !ids[id] {
				delete(retries, id)
			}
		}

		now := time.Now()
		for _, id := range slices.Sorted(maps.Keys(ids)) {
			r, failed := retries[id]
			if subscribed[id] || failed && now.Before(r.at) {
				continue
			}
			raw, err := c.request(ctx, methodSubscribeUnit, unitParams{SysID: c.SystemID(), ID: string(id)})
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				r.delay = min(max(2*r.delay, firstSubscribeRetry), maxSubscribeRetry)
				r.at = time.Now().Add(r.delay)
				retries[id] = r
				c.logger.Info("unit configuration is not available, retrying",
					slog.String("unit", string(id)), slog.Duration("in", r.delay), slog.Any("error", err))
				continue
			}
			delete(retries, id)
			subscribed[id] = true
			select {
			case results <- configResult{id: id, raw: raw}:
			case <-ctx.Done():
				return
			}
		}

		// Wake up for the next retry.
		wake = nil
		if len(retries) > 0 {
			next := slices.MinFunc(slices.Collect(maps.Values(retries)), func(a, b retry) int { return a.at.Compare(b.at) })
			if timer == nil {
				timer = time.NewTimer(time.Until(next.at))
			} else {
				timer.Reset(time.Until(next.at))
			}
			wake = timer.C
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
