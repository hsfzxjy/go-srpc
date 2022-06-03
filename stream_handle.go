package srpc

import (
	"sync/atomic"
)

type streamHandle struct {
	sid    uint64
	state  uint32
	client *client

	isPolling uint32

	Err   error
	Panic *panicInfo
	ch    chan any
}

func (h *streamHandle) startPoll() error {
	var flushed []*StreamEvent

	for h.state&ssFinished == 0 {
		flushed = nil
		err := h.client.Call("StreamManager.Poll", h.sid, &flushed)
		if h.state&ssFinished != 0 {
			return nil
		}
		if err != nil {
			panic(err)
		}

		if len(flushed) > 0 && flushed[len(flushed)-1].Typ.IsTerminal() {
			h.state |= ssFinished
		}

	LOOP:
		for _, e := range flushed {

			switch e.Typ {
			case seValue:
				h.ch <- e.Data
				continue LOOP
			case seLog:
				clientLogFunc(e.Data.(string))
				continue LOOP
			case seError:
				if e.Data != nil {
					h.Err = e.Data.(error)
				}
			case sePanic:
				var pi = e.Data.(panicInfo)
				if _, ok := pi.Data.(*sessionError); ok {
					h.Err = &pi
				} else {
					h.Panic = &pi
				}
			case seDone:
			}
			close(h.ch)
		}
	}

	return nil
}

func (h *streamHandle) ensurePolling() {
	if atomic.LoadUint32(&h.isPolling) == 1 {
		return
	}
	if atomic.CompareAndSwapUint32(&h.isPolling, 0, 1) {
		go h.startPoll()
	}
}

func (h *streamHandle) C() <-chan any {
	h.ensurePolling()

	return h.ch
}

func (h *streamHandle) Success() bool {
	if h.state&ssFinished == 0 {
		panic("srpc: Success() called before stream finished")
	}
	return h.Panic == nil && h.Err == nil
}

func (h *streamHandle) Result() error {
	if h.state&ssFinished == 0 {
		panic("srpc: Result() called before stream finished")
	}
	if h.Panic != nil {
		panic(h.Panic)
	}
	return h.Err
}

func (h *streamHandle) Cancel() bool {
	if h.state&ssCanceled != 0 {
		panic("srpc: Cancel() called on an already canceled stream")
	}

	var reply bool
	h.state |= ssCanceled | ssFinished
	h.client.Call("StreamManager.Cancel", h.sid, &reply)
	return reply
}
