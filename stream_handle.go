package srpc

import (
	"sync/atomic"
)

type StreamHandle struct {
	sid        uint64
	state      sessionState
	client     *Client
	finishedCh chan struct{}

	isPolling uint32

	Err   error
	Panic *panicInfo
	ch    chan any
}

func newStreamHandle(client *Client) *StreamHandle {
	handle := new(StreamHandle)
	handle.sid = 0
	handle.ch = make(chan any)
	handle.client = client
	handle.finishedCh = make(chan struct{})
	return handle
}

func (h *StreamHandle) startPoll() error {
	var flushed []*StreamEvent

	for !h.state.hasFlagLock(ssFinished) {
		flushed = nil
		err := h.client.Call("StreamManager.Poll", h.sid, &flushed)
		if h.state.hasFlagLock(ssFinished) {
			return nil
		}
		if err != nil {
			panic(err)
		}

		if len(flushed) > 0 && flushed[len(flushed)-1].Typ.IsTerminal() {
			h.state.setFlagLock(ssFinished)
			close(h.finishedCh)
		}

	LOOP:
		for _, e := range flushed {
			switch e.Typ {
			case seValue:
				h.state.L.RLock()
				if h.state.hasFlag(ssCanceled) {
					h.state.L.RUnlock()
					break LOOP
				}
				h.ch <- e.Data
				h.state.L.RUnlock()
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

func (h *StreamHandle) ensurePolling() {
	if atomic.LoadUint32(&h.isPolling) == 1 {
		return
	}
	if atomic.CompareAndSwapUint32(&h.isPolling, 0, 1) {
		go h.startPoll()
	}
}

func (h *StreamHandle) C() <-chan any {
	h.ensurePolling()

	return h.ch
}

func (h *StreamHandle) Success() bool {
	if !h.state.hasFlagLock(ssFinished) {
		panic("srpc: Success() called before stream finished")
	}
	return h.Panic == nil && h.Err == nil
}

func (h *StreamHandle) Result() error {
	if !h.state.hasFlagLock(ssFinished) {
		panic("srpc: Result() called before stream finished")
	}
	if h.Panic != nil {
		panic(h.Panic)
	}
	return h.Err
}

func (h *StreamHandle) CancelAndResult() error {
	h.Cancel()
	return h.Result()
}

func (h *StreamHandle) GetError() error {
	if !h.state.hasFlagLock(ssFinished) {
		panic("srpc: GetError() called before stream finished")
	}
	if h.Err != nil {
		return h.Err
	} else {
		return h.Panic
	}
}

func (h *StreamHandle) Cancel() bool {
	if h.state.hasFlagLock(ssCanceled | ssFinished) {
		return false
	}

	var reply bool
	h.state.L.Lock()
	defer h.state.L.Unlock()
	h.state.setFlag(ssCanceled | ssFinished)
	h.client.Call("StreamManager.Cancel", h.sid, &reply)
	close(h.ch)

	return reply
}

func (h *StreamHandle) IsFinished() bool {
	return h.state.hasFlagLock(ssFinished)
}

func (h *StreamHandle) BlockUntilFinished() {
	<-h.finishedCh
}
