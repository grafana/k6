package websocket

import "errors"

var (
	ErrNilConn    = errors.New("nil *Conn")
	ErrNilNetConn = errors.New("nil net.Conn")
)
