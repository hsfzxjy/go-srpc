package srpc

import (
	"encoding/gob"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hsfzxjy/mamo"
)

// A streaming RPC session
type Session struct {
	Sid uint64

	buf chan *StreamEvent
	// signal that no more events should be pushed
	endCh    chan struct{}
	endOnce  sync.Once
	EndCause EndCause

	// signal that the main function has finished
	doneCh chan struct{}

	// signal that all events be flushed
	flushedCh   chan struct{}
	flushedOnce sync.Once

	finalEvent chan *StreamEvent

	cfg *SessionConfig

	// for checking client timeout
	mamo *mamo.Mamo
}

func (s *Session) initSession(sid uint64, cfg *SessionConfig) {
	cfg = mergeConfig(cfg)
	s.Sid = sid
	s.buf = make(chan *StreamEvent, cfg.BufferCapacity)
	s.endCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.flushedCh = make(chan struct{})
	s.finalEvent = make(chan *StreamEvent, 1)
	s.cfg = cfg
	s.mamo = mamo.New(s.cfg.ClientTimeout, func() bool {
		s.pushError(EC_CLIENT_TIMEOUT)
		return true
	})
}

func (s *Session) markEnded(cause EndCause) bool {
	var marked bool
	s.endOnce.Do(func() {
		s.EndCause = cause
		close(s.endCh)
		marked = true
		go s.mamo.Quit()
	})
	return marked
}

func (s *Session) push(typ streamEventType, data any) {
	event := &StreamEvent{Typ: typ, Data: data}

	if typ.IsTerminal() {
		select {
		case s.finalEvent <- event:
		default:
		}
	} else {
		select {
		case s.buf <- event:
		case <-s.endCh:
			panic(s.EndCause)
		}
	}
}

func (s *Session) pushError(e error) {
	var cause = EC_ERROR
	if c, ok := e.(EndCause); ok {
		cause = c
	}
	if !s.markEnded(cause) {
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

// Push arbitary value to client
func (s *Session) PushValue(data any) {
	s.push(seValue, data)
}

// Log a formatted message at client side
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
		if s.cfg.PollTimeout > 0 {
			select {
			case <-s.doneCh:
			case <-time.After(s.cfg.PollTimeout):
				return nil
			}
		} else {
			<-s.doneCh
		}

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

		cond := time.After(50 * time.Millisecond)
	LOOP2:
		for {
			select {
			case event := <-s.buf:
				ret = append(ret, event)
			case <-cond:
				break LOOP2
			}
		}
	}

	return ret
}

func (s *Session) cancel() {
	s.pushError(EC_CLIENT_CANCELED)
}

func (s *Session) EndC() <-chan struct{} {
	return s.endCh
}

func init() {
	gob.Register(new(Session))
}
