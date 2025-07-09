package main

import (
	"context"

	"github.com/tgoodwin/monolift/demo/monolith/postservice"
	"github.com/tgoodwin/monolift/demo/monolith/types/post"
	"github.com/tgoodwin/monolift/pkg/pragma"
)

// postserviceClientDelegate is a client that can delegate calls to a remote service
// based on the decision of a pragma.Decider.
type postserviceClientDelegate struct {
	local   postservice.Service
	remote  postservice.Service
	decider pragma.Decider
}

// NewpostserviceClientDelegate creates a new delegate client.
// It takes a local implementation, a remote client, and a decider which determines
// when to use the remote client.
func NewpostserviceClientDelegate(local, remote postservice.Service, decider pragma.Decider) *postserviceClientDelegate {
	return &postserviceClientDelegate{
		local:   local,
		remote:  remote,
		decider: decider,
	}
}

// AddComment delegates the call to either the local or remote implementation.
func (d *postserviceClientDelegate) AddComment(ctx context.Context, req post.CommentReq) (post.UpdatePostResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.AddComment(ctx, req)
	}
	return d.local.AddComment(ctx, req)
}

// DeletePost delegates the call to either the local or remote implementation.
func (d *postserviceClientDelegate) DeletePost(ctx context.Context, req post.DelPostReq) (post.UpdatePostResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.DeletePost(ctx, req)
	}
	return d.local.DeletePost(ctx, req)
}

// ReadPosts delegates the call to either the local or remote implementation.
func (d *postserviceClientDelegate) ReadPosts(ctx context.Context, req post.ReadPostReq) (post.ReadPostResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.ReadPosts(ctx, req)
	}
	return d.local.ReadPosts(ctx, req)
}

// SavePost delegates the call to either the local or remote implementation.
func (d *postserviceClientDelegate) SavePost(ctx context.Context, req post.SavePostReq) (post.UpdatePostResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.SavePost(ctx, req)
	}
	return d.local.SavePost(ctx, req)
}

// UpdateMeta delegates the call to either the local or remote implementation.
func (d *postserviceClientDelegate) UpdateMeta(ctx context.Context, req post.MetaReq) (post.UpdatePostResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.UpdateMeta(ctx, req)
	}
	return d.local.UpdateMeta(ctx, req)
}

// UpvotePost delegates the call to either the local or remote implementation.
func (d *postserviceClientDelegate) UpvotePost(ctx context.Context, req post.UpvoteReq) (post.UpdatePostResp, error) {
	if d.decider.ShouldDelegate() {
		return d.remote.UpvotePost(ctx, req)
	}
	return d.local.UpvotePost(ctx, req)
}
