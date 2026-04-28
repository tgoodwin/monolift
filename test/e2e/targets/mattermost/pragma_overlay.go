//go:build monolift_e2e

package platform

import real "github.com/mattermost/mattermost/server/v8/channels/app/platform"

//monolift:lift name=connection-hub-buffer mode=remote transport=http-json methods=Start,Broadcast,Register,Unregister,CheckConn,SendMessage,ProcessAsync,Stop
type Hub = real.Hub

//monolift:lift name=connection-hub-buffer mode=remote transport=http-json methods=Pump,writePump
type WebConn = real.WebConn
