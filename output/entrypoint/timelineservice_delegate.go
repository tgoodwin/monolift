package main

import (
	"context"

	"github.com/tgoodwin/monolift/demo/monolith/timelineservice"
	"github.com/tgoodwin/monolift/demo/monolith/types/timeline"
	"github.com/tgoodwin/monolift/pkg/pragma"
)

// timelineserviceClientDelegate is a client that can delegate calls to a remote service
// based on the decision of a pragma.Decider.
type timelineserviceClientDelegate struct {
	local   timelineservice.Service
	remote  timelineservice.Service
	decider pragma.Decider
}

// NewtimelineserviceClientDelegate creates a new delegate client.
// It takes a local implementation, a remote client, and a decider which determines
// when to use the remote client.
func NewtimelineserviceClientDelegate(local, remote timelineservice.Service, decider pragma.Decider) *timelineserviceClientDelegate {
	return &timelineserviceClientDelegate{
		local:   local,
		remote:  remote,
		decider: decider,
	}
}

// ReadTimeline delegates the call to either the local or remote implementation.
func (d *timelineserviceClientDelegate) ReadTimeline(ctx context.Context, req timeline.ReadReq) (timeline.ReadResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.ReadTimeline(ctx, req)
	}
	return d.local.ReadTimeline(ctx, req)
}

// UpdateTimeline delegates the call to either the local or remote implementation.
func (d *timelineserviceClientDelegate) UpdateTimeline(ctx context.Context, req timeline.UpdateReq) (timeline.UpdateResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.UpdateTimeline(ctx, req)
	}
	return d.local.UpdateTimeline(ctx, req)
}
