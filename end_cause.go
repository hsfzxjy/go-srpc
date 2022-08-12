package srpc

import "encoding/gob"


type EndCause int

const (
	EC_UNKNOWN EndCause = iota
	EC_NORMAL
	EC_ERROR
	EC_PANIC
	EC_CLIENT_CANCELED
	EC_CLIENT_TIMEOUT
)

func (ec EndCause) Error() string {
	switch ec {
	case EC_UNKNOWN:
		return "srpc-server: session has ended due to unknown reason"
	case EC_NORMAL:
		return "srpc-server: session has ended normally"
	case EC_ERROR:
		return "srpc-server: session has ended due to server-side error"
	case EC_PANIC:
		return "srpc-server: session has ended due to server-side panic"
	case EC_CLIENT_CANCELED:
		return "srpc-server: session has ended due to client-side cancellation"
	case EC_CLIENT_TIMEOUT:
		return "srpc-server: session has ended due to client timeout"
	default:
		panic("unreachable")
	}
}

func init() {
	gob.Register(EndCause(0))
}
