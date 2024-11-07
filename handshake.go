package websocket

import (
	"net/http"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type HandshakeError struct {
	message string
}

func (e HandshakeError) Error() string { return e.message }

type UpgradeConn struct {
}

func (u *UpgradeConn) checkHandshake() {

}

func (u *UpgradeConn) ReadPacket(w *http.ResponseWriter, r *http.Request)
