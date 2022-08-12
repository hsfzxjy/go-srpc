package srpc

import (
	"encoding/gob"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Session struct {
	Sid uint64

	buf      chan *StreamEvent
	endCh    chan struct{}
	endOnce  sync.Once
	EndCause EndCause

	flushedCh   chan struct{}
	flushedOnce sync.Once

	finalEvent chan *StreamEvent

	cfg *SessionConfig

	idleDetector *idleDetector
}

func (s *Session) initSession(sid uint64, cfg *SessionConfig) {
	cfg = mergeConfig(cfg)
	s.Sid = sid
	s.buf = make(chan *StreamEvent, cfg.BufferCapacity)
	s.endCh = make(chan struct{})
	s.flushedCh = make(chan struct{})
	s.finalEvent = make(chan *StreamEvent, 1)
	s.cfg = cfg
	s.idleDetector = newIdleDetector(s.cfg.ClientTimeout, func() {
		s.pushError(EC_CLIENT_TIMEOUT)
	})
}

func (s *Session) markEnded(cause EndCause) bool {
	var marked bool
	s.endOnce.Do(func() {
		s.EndCause = cause
		close(s.endCh)
		marked = true
		s.idleDetector.push(idQuit)
	})
	return marked
}

func (s *Session) push(typ streamEventType, data any) {
	event := &StreamEvent{Typ: typ, Data: data}

	if typ.IsTerminal() {
		s.finalEvent <- event
	} else {
		select {
		case s.buf <- event:
		case <-s.endCh:
			panic(s.EndCause)
		}
	}
}

func (s *Session) pushError(e error) {
	if !s.markEnded(EC_ERROR) {
		return
	}
	e = transformError(e)
	s.push(seError, e)
}

func (s *Session) pushDone() {
	if !s.markEnded(EC_NORMAL) {
		return
	}
	s.push(seDone, nil)
}

func (s *Session) pushPanic(val *panicInfo) {
	if !s.markEnded(EC_PANIC) {
		return
	}
	if err, ok := val.Data.(error); ok {
		val.Data = transformError(err)
	}
	s.push(sePanic, val)
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

func (s *Session) waitFlush(timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-s.flushedCh:
	case <-timer.C:
	}
}

func (s *Session) flush() (ret []*StreamEvent) {
	var ended bool
	select {
	case <-s.endCh:
		ended = true
	default:
	}

	var finalEvent *StreamEvent
	select {
	case finalEvent = <-s.finalEvent:
	default:
	}

	ret = make([]*StreamEvent, 0, 16)

	if ended {
		if finalEvent == nil {
			<-s.flushedCh
			return nil
		} else {
		LOOP:
			// flush out all events
			for {
				select {
				case event := <-s.buf:
					ret = append(ret, event)
				default:
					break LOOP
				}
			}
			ret = append(ret, finalEvent)
			s.flushedOnce.Do(func() { close(s.flushedCh) })
		}
	} else {
		// block until the first event arrived
		var breakFirstPoll chan struct{} = s.endCh
		if s.cfg.PollTimeout > 0 {
			breakFirstPoll = make(chan struct{})
			go func() {
				select {
				case <-s.endCh:
				case <-time.After(s.cfg.PollTimeout):
				}
				close(breakFirstPoll)
			}()
		}

		select {
		case first := <-s.buf:
			ret = append(ret, first)
		case <-breakFirstPoll:
			return nil
		}

	LOOP2:
		for {
			select {
			case event := <-s.buf:
				ret = append(ret, event)
			default:
				break LOOP2
			}
		}
	}

	return ret
}

func (s *Session) cancel() {
	s.markEnded(EC_CLIENT_CANCELED)
}

func (s *Session) EndedC() <-chan struct{} {
	return s.endCh
}

func init() {
	gob.Register(new(Session))
}
