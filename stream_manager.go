package srpc

import (
	"errors"
	"net/rpc"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

var errNoSuchSession = errors.New("srpc-server: no such session")

type StreamManager struct {
	id       uint64
	sessions sync.Map
}

var manager = StreamManager{id: 0}

func S(f func() error, sess *Session, cfg *SessionConfig) error {

	sid := atomic.AddUint64(&manager.id, 1)
	sess.initSession(sid, cfg)

	manager.sessions.Store(sid, sess)

	go func() {
		defer func() {
			if p := recover(); p != nil {
				serverLogFunc("%+v\n", p)
				sess.pushPanic(&panicInfo{
					Data:  p,
					Stack: debug.Stack(),
				})
			}
			sess.waitFlush(sess.cfg.KeepAlive)
			manager.sessions.Delete(sid)
		}()
		sess.mamo.Loop()
		err := f()
		if err == nil {
			sess.pushDone()
		} else {
			sess.pushError(err)
		}
	}()

	return nil
}

func (m *StreamManager) Poll(sid uint64, reply *[]*StreamEvent) error {
	v, loaded := manager.sessions.Load(sid)
	if !loaded {
		return errNoSuchSession
	}
	sess := v.(*Session)
	sess.mamo.Acquire()
	defer sess.mamo.Release()
	*reply = sess.flush()
	return nil
}

func (m *StreamManager) Cancel(sid uint64, loaded *bool) error {
	var sess any
	sess, *loaded = manager.sessions.LoadAndDelete(sid)
	if !*loaded {
		return nil
	}
	sess.(*Session).cancel()

	return nil
}

func (m *StreamManager) SoftCancel(sid uint64, loaded *bool) error {
	var sess any
	sess, *loaded = manager.sessions.Load(sid)
	if !*loaded {
		return nil
	}
	sess.(*Session).cancel()

	return nil
}

func init() {
	rpc.Register(&manager)
}
