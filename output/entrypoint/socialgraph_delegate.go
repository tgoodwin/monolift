package main

import (
	"context"

	"github.com/tgoodwin/monolift/demo/monolith/socialgraph"
	"github.com/tgoodwin/monolift/pkg/pragma"
)

// socialgraphClientDelegate is a client that can delegate calls to a remote service
// based on the decision of a pragma.Decider.
type socialgraphClientDelegate struct {
	local   socialgraph.Service
	remote  socialgraph.Service
	decider pragma.Decider
}

// NewsocialgraphClientDelegate creates a new delegate client.
// It takes a local implementation, a remote client, and a decider which determines
// when to use the remote client.
func NewsocialgraphClientDelegate(local, remote socialgraph.Service, decider pragma.Decider) *socialgraphClientDelegate {
	return &socialgraphClientDelegate{
		local:   local,
		remote:  remote,
		decider: decider,
	}
}

// Follow delegates the call to either the local or remote implementation.
func (d *socialgraphClientDelegate) Follow(ctx context.Context, req socialgraph.FollowReq) (socialgraph.UpdateResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.Follow(ctx, req)
	}
	return d.local.Follow(ctx, req)
}

// GetFollowees delegates the call to either the local or remote implementation.
func (d *socialgraphClientDelegate) GetFollowees(ctx context.Context, req socialgraph.GetReq) (socialgraph.GetFollowResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.GetFollowees(ctx, req)
	}
	return d.local.GetFollowees(ctx, req)
}

// GetFollowers delegates the call to either the local or remote implementation.
func (d *socialgraphClientDelegate) GetFollowers(ctx context.Context, req socialgraph.GetReq) (socialgraph.GetFollowerResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.GetFollowers(ctx, req)
	}
	return d.local.GetFollowers(ctx, req)
}

// GetRecommendations delegates the call to either the local or remote implementation.
func (d *socialgraphClientDelegate) GetRecommendations(ctx context.Context, req socialgraph.GetRecmdReq) (socialgraph.GetRecmdResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.GetRecommendations(ctx, req)
	}
	return d.local.GetRecommendations(ctx, req)
}

// Unfollow delegates the call to either the local or remote implementation.
func (d *socialgraphClientDelegate) Unfollow(ctx context.Context, req socialgraph.UnfollowReq) (socialgraph.UpdateResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.Unfollow(ctx, req)
	}
	return d.local.Unfollow(ctx, req)
}
