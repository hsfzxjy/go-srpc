package srpc

import (
	"encoding/gob"
	"fmt"
	"strings"
	"sync"
	"time"
)

type SessionConfig struct {
	BufferCapacity int
	ClientTimeout  time.Duration
	KeepAlive      time.Duration
}

func (cfg *SessionConfig) validate() bool {
	return cfg.BufferCapacity >= 0 && cfg.ClientTimeout > 0 && cfg.KeepAlive > 0
}

func (cfg *SessionConfig) copyFrom(src *SessionConfig, strict bool) bool {
	if src == nil {
		return false
	}
	if strict && !src.validate() {
		return false
	}

	if src.BufferCapacity >= 0 {
		cfg.BufferCapacity = src.BufferCapacity
	}

	if src.ClientTimeout > 0 {
		cfg.ClientTimeout = src.ClientTimeout
	}

	if src.KeepAlive > 0 {
		cfg.KeepAlive = src.KeepAlive
	}

	return true
}

func mergeConfig(cfg *SessionConfig) (ret *SessionConfig) {
	ret = &SessionConfig{}
	ret.copyFrom(&defaultSessionConfig, false)
	ret.copyFrom(cfg, false)
	return
}

type sessionError struct{ S string }

func newSessionError(str string) *sessionError { return &sessionError{S: str} }
func (e *sessionError) Error() string          { return e.S }

func init() {
	gob.Register(errClientTimeout)
}

var errClientTimeout = newSessionError("srpc-server: client push timeout")
var errSessionFinished = newSessionError("srpc-server: session has finished")
var errSessionCanceled = newSessionError("srpc-server: session was canceled by client")

type Session struct {
	Sid   uint64
	state sessionState
	l     *sync.Mutex

	buf      chan *StreamEvent
	endCh    chan struct{}
	cancelCh chan struct{}

	panicInfo *panicInfo

	cfg *SessionConfig
}

func (s *Session) initSession(sid uint64, cfg *SessionConfig) {
	cfg = mergeConfig(cfg)
	s.Sid = sid
	s.state.state = 0
	s.l = &sync.Mutex{}
	s.buf = make(chan *StreamEvent, cfg.BufferCapacity)
	s.endCh = make(chan struct{})
	s.cancelCh = make(chan struct{})
	s.panicInfo = nil
	s.cfg = cfg
}

func (s *Session) sessionErrorFromState() error {
	s.state.L.RLock()
	defer s.state.L.RUnlock()
	if s.state.hasFlag(ssCanceled) {
		return errSessionCanceled
	} else if s.state.hasFlag(ssTimeout) {
		return errClientTimeout
	} else {
		return errSessionFinished
	}
}

func (s *Session) push(typ streamEventType, data any) {
	if s.state.hasFlagLock(ssFinished) {
		panic(s.sessionErrorFromState())
	}

	timer := time.NewTimer(s.cfg.ClientTimeout)
	defer timer.Stop()

	event := &StreamEvent{Typ: typ, Data: data}

	if typ.IsTerminal() {
		s.state.setFlagLock(ssFinished)
	}

	select {
	case s.buf <- event:
	case <-s.cancelCh:
		panic(errSessionCanceled)
	case <-timer.C:
		s.state.setFlagLock(ssFinished | ssTimeout)
		panic(errClientTimeout)
	}
}

func (s *Session) pushError(e error) {
	e = transformError(e)
	s.push(seError, e)
}

func (s *Session) PushValue(data any) {
	s.push(seValue, data)
}

func (s *Session) Logf(format string, args ...any) {
	s.push(seLog, fmt.Sprintf(format, args...))
}

func (s *Session) Logv(args ...any) {
	builder := strings.Builder{}
	for idx, arg := range args {
		if idx > 0 {
			builder.WriteRune(' ')
		}
		builder.Write([]byte(fmt.Sprintf("%+v", arg)))
	}
	s.push(seLog, builder.String())
}

func (s *Session) done() {
	s.push(seDone, nil)
}

func (s *Session) panic(val *panicInfo) {
	if s.state.hasFlagLock(ssFinished) {
		s.panicInfo = val
		return
	}
	s.push(sePanic, val)
}

func (s *Session) waitFlush(timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-s.endCh:
	case <-timer.C:
	}
}

func (s *Session) flush() (ret []*StreamEvent) {
	if s.state.hasFlagLock(ssFinished) {
		if s.cfg.BufferCapacity > 0 {
			if len(s.buf) == 0 {
				return nil
			}
		} else {
			if s.panicInfo != nil {
				ret = []*StreamEvent{
					{Typ: sePanic, Data: s.panicInfo},
				}
			}
			close(s.endCh)
			return
		}
	}

	ret = make([]*StreamEvent, 0, 16)
	select {
	case e := <-s.buf:
		ret = append(ret, e)
	case <-s.endCh:
		return nil
	}
	for len(s.buf) > 0 {
		ret = append(ret, <-s.buf)
	}

	if s.state.hasFlagLock(ssFinished) {
		if s.panicInfo != nil {
			ret = append(ret, &StreamEvent{Typ: sePanic, Data: s.panicInfo})
		}
		close(s.endCh)
	}

	return ret
}

func (s *Session) cancel() {
	s.state.L.Lock()
	defer s.state.L.Unlock()
	s.state.setFlag(ssFinished | ssCanceled)
	close(s.cancelCh)
}

func (s *Session) Canceled() <-chan struct{} {
	return s.cancelCh
}

func init() {
	gob.Register(Session{})
}
