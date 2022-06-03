package srpc

import (
	"errors"
	"net/rpc"
	"runtime/debug"
	"sync"
)

var errNoSuchSession = errors.New("srpc-server: no such session")

type StreamManager struct {
	id       uint64
	lock     sync.Mutex
	sessions map[uint64]*Session
}

var manager = StreamManager{
	id:       0,
	lock:     sync.Mutex{},
	sessions: make(map[uint64]*Session),
}

func S(f func() error, sess *Session, cfg *SessionConfig) error {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	sid := manager.id
	sess.initSession(sid, cfg)
	manager.sessions[sid] = sess
	manager.id++

	go func() {
		defer func() {
			if p := recover(); p != nil {
				if sess.state&ssCanceled != 0 {
					serverLogFunc("%+v\n", p)
					return
				}
				sess.panic(&panicInfo{Data: p, Stack: debug.Stack()})
			}
			sess.waitFlush(sess.cfg.KeepAlive)
			manager.lock.Lock()
			defer manager.lock.Unlock()
			delete(manager.sessions, sid)
		}()
		err := f()
		if err == nil {
			sess.done()
		} else {
			sess.pushError(err)
		}
	}()

	return nil
}

func (m *StreamManager) Poll(sid uint64, reply *[]*StreamEvent) error {
	manager.lock.Lock()
	sess, ok := manager.sessions[sid]
	manager.lock.Unlock()
	if !ok {
		return errNoSuchSession
	}

	*reply = sess.flush()
	return nil
}

func (m *StreamManager) Cancel(sid uint64, reply *bool) error {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	var sess *Session
	sess, *reply = manager.sessions[sid]
	if !*reply {
		return nil
	}
	sess.cancel()
	delete(manager.sessions, sess.Sid)

	return nil
}

func init() {
	rpc.Register(&manager)
}
