package agent

import (
	"net/http"

	"github.com/gorilla/websocket"

	"tsukiyo/master/internal/api/middleware"
)

func wsUpgrader(subprotocols ...string) websocket.Upgrader {
	subs := subprotocols
	return websocket.Upgrader{
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
		Subprotocols:    subs,
		CheckOrigin: func(r *http.Request) bool {
			return middleware.IsAllowedOrigin(r)
		},
	}
}

var (
	upgrader    = wsUpgrader()
	vncUpgrader = wsUpgrader("binary")
)
