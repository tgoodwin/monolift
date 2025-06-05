package frontend

// SaveReq is the request to save a post, specific to the frontend API.
type SaveReq struct {
	UserId        string   `json:"user_id"`
	Text          string   `json:"text"`
	Images        []string `json:"images"` // base64 encoded images
	SendUnixMilli int64    `json:"send_unix_milli"`
}

// UpdateResp is the response for update operations (save, del, comment, upvote), specific to the frontend API.
type UpdateResp struct {
	PostId string `json:"post_id"`
}

// DelReq is the request to delete a post, specific to the frontend API.
type DelReq struct {
	UserId        string `json:"user_id"`
	PostId        string `json:"post_id"`
	SendUnixMilli int64  `json:"send_unix_milli"`
}

// FrontendCommentAPIRq is the request to add a comment received at the frontend API layer.
type FrontendCommentAPIRq struct {
	PostId        string `json:"post_id"`
	UserId        string `json:"user_id"`
	ReplyTo       string `json:"reply_to"` // can be empty, means reply to post
	Text          string `json:"text"`
	SendUnixMilli int64  `json:"send_unix_milli"`
}

// ImageReq is the request to get an image, specific to the frontend API.
type ImageReq struct {
	Image         string `json:"image"` // image id
	SendUnixMilli int64  `json:"send_unix_milli"`
}
