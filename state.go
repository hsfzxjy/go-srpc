package srpc

import "sync"

const (
	ssFinished uint32 = 1 << iota
	ssTimeout
	ssCanceled
)

type sessionState struct {
	state uint32
	L     sync.RWMutex
}

func (ss *sessionState) hasFlag(flag uint32) bool {
	return ss.state&flag != 0
}

func (ss *sessionState) hasFlagLock(flag uint32) bool {
	ss.L.RLock()
	defer ss.L.RUnlock()
	return ss.state&flag != 0
}

func (ss *sessionState) setFlag(flag uint32) {
	ss.state |= flag
}

func (ss *sessionState) setFlagLock(flag uint32) {
	ss.L.Lock()
	defer ss.L.Unlock()
	ss.state |= flag
}
