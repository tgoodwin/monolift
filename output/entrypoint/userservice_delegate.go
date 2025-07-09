package main

import (
	"context"

	"github.com/tgoodwin/monolift/demo/monolith/types/user"
	"github.com/tgoodwin/monolift/demo/monolith/userservice"
	"github.com/tgoodwin/monolift/pkg/pragma"
)

// userserviceClientDelegate is a client that can delegate calls to a remote service
// based on the decision of a pragma.Decider.
type userserviceClientDelegate struct {
	local   userservice.Service
	remote  userservice.Service
	decider pragma.Decider
}

// NewuserserviceClientDelegate creates a new delegate client.
// It takes a local implementation, a remote client, and a decider which determines
// when to use the remote client.
func NewuserserviceClientDelegate(local, remote userservice.Service, decider pragma.Decider) *userserviceClientDelegate {
	return &userserviceClientDelegate{
		local:   local,
		remote:  remote,
		decider: decider,
	}
}

// Login delegates the call to either the local or remote implementation.
func (d *userserviceClientDelegate) Login(ctx context.Context, req user.LoginReq) (user.LoginResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.Login(ctx, req)
	}
	return d.local.Login(ctx, req)
}

// Register delegates the call to either the local or remote implementation.
func (d *userserviceClientDelegate) Register(ctx context.Context, req user.RegisterReq) (user.RegisterResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.Register(ctx, req)
	}
	return d.local.Register(ctx, req)
}
