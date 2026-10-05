package xlinkclient

import "context"

// Start starts a unit. The unit may belong to a remote system.
func (c *Client) Start(ctx context.Context, id UnitID) error {
	return c.callUnit(ctx, methodStart, c.SystemID(), string(id), nil)
}

// Stop stops a unit. The unit may belong to a remote system.
func (c *Client) Stop(ctx context.Context, id UnitID) error {
	return c.callUnit(ctx, methodStop, c.SystemID(), string(id), nil)
}

// ResetStats resets the statistics of a unit ("Reset Stats").
func (c *Client) ResetStats(ctx context.Context, id UnitID) error {
	return c.callUnit(ctx, methodResetStats, c.SystemID(), string(id), nil)
}

// ResetVideoBuffer resets the video buffer of a running XLink encoder
// ("Reset Buffer").
func (c *Client) ResetVideoBuffer(ctx context.Context, id UnitID) error {
	return c.callUnit(ctx, methodResetBuffer, c.SystemID(), string(id), nil)
}

// FlushAudio resets the audio buffer of a running XLink decoder
// ("Reset Audio Buffer").
func (c *Client) FlushAudio(ctx context.Context, id UnitID) error {
	return c.callUnit(ctx, methodFlushAudio, c.SystemID(), string(id), nil)
}

// CreateUnit creates a unit of the given type on the local system. The device
// does not return the ID of the new unit; it appears in the next snapshot.
func (c *Client) CreateUnit(ctx context.Context, typ UnitType) error {
	return c.callSystem(ctx, methodNewVideo, map[string]any{"type": int(typ)})
}

// DeleteUnit deletes a unit of the local system. The device reuses IDs of
// deleted units.
func (c *Client) DeleteUnit(ctx context.Context, id UnitID) error {
	return c.callUnit(ctx, methodDeleteVideo, c.SystemID(), string(id), nil)
}

// CreateTrunk creates a layer 2 trunk on the given system, which may be a
// remote one. The device does not return the ID of the new trunk; it appears
// in the next snapshot.
func (c *Client) CreateTrunk(ctx context.Context, system SystemID) error {
	return c.call(ctx, methodNewTrunk, systemParams{SysID: system})
}

// StartTrunk starts a layer 2 trunk.
func (c *Client) StartTrunk(ctx context.Context, id TrunkID) error {
	return c.callUnit(ctx, methodStartTrunk, id.System(), string(id), nil)
}

// StopTrunk stops a layer 2 trunk.
func (c *Client) StopTrunk(ctx context.Context, id TrunkID) error {
	return c.callUnit(ctx, methodStopTrunk, id.System(), string(id), nil)
}

// DeleteTrunk deletes a layer 2 trunk, which may belong to a remote system.
func (c *Client) DeleteTrunk(ctx context.Context, id TrunkID) error {
	return c.callUnit(ctx, methodDeleteTrunk, id.System(), string(id), nil)
}
