package srpc

import (
	"net/rpc"
	"sync"
	"sync/atomic"
)

type StreamHandle struct {
	sid        uint64
	client     *Client
	endedCh    chan struct{}
	isCanceled int32
	endedOnce  sync.Once
	EndCause   EndCause

	pollOnce sync.Once

	Err   error
	Panic *panicInfo
	ch    chan any
}

func newStreamHandle(client *Client) *StreamHandle {
	handle := new(StreamHandle)
	handle.sid = 0
	handle.ch = make(chan any)
	handle.client = client
	handle.endedCh = make(chan struct{})
	return handle
}

func (h *StreamHandle) markEnded() {
	h.endedOnce.Do(func() {
		close(h.endedCh)
		close(h.ch)
	})
}

func (h *StreamHandle) startPoll() error {
	var flushed []*StreamEvent

	for {
		select {
		case <-h.endedCh:
			return nil
		default:
		}

		flushed = nil
		invokeCh := make(chan *rpc.Call, 1)
		call := h.client.Go("StreamManager.Poll", h.sid, &flushed, invokeCh)
		select {
		case <-h.endedCh:
			return nil
		case <-invokeCh:
		}

		if call.Error != nil {
			panic(call.Error)
		}

	DISPATCH_EVENTS:
		for _, e := range flushed {
			switch e.Typ {
			case seValue:
				select {
				case h.ch <- e.Data:
					continue DISPATCH_EVENTS
				case <-h.endedCh:
				}
			case seLog:
				clientLogFunc(e.Data.(string))
				continue DISPATCH_EVENTS
			case seError:
				if e.Data != nil {
					h.Err = e.Data.(error)
				}
			case sePanic:
				var pi = e.Data.(panicInfo)
				if _, ok := pi.Data.(EndCause); ok {
					h.Err = &pi
				} else {
					h.Panic = &pi
				}
			case seDone:
			}
			h.markEnded()
		}
	}
}

func (h *StreamHandle) ensurePolling() {
	h.pollOnce.Do(func() {
		go h.startPoll()
	})
}

func (h *StreamHandle) C() <-chan any {
	h.ensurePolling()

	return h.ch
}

func (h *StreamHandle) Success() bool {
	h.ensurePolling()
	<-h.endedCh
	return h.Panic == nil && h.Err == nil
}

func (h *StreamHandle) Result() error {
	h.ensurePolling()
	<-h.endedCh
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
	h.ensurePolling()
	<-h.endedCh
	if h.Err != nil {
		return h.Err
	} else {
		return h.Panic
	}
}

func (h *StreamHandle) SoftCancel() bool {
	var reply bool
	if atomic.CompareAndSwapInt32(&h.isCanceled, 0, 1) {
		h.client.Call("StreamManager.SoftCancel", h.sid, &reply)
	}
	return reply
}

func (h *StreamHandle) Cancel() bool {
	select {
	case <-h.endedCh:
		return false
	default:
	}

	h.markEnded()
	h.Err = EC_CLIENT_CANCELED

	return h.SoftCancel()
}

func (h *StreamHandle) IsEnded() bool {
	h.ensurePolling()
	select {
	case <-h.endedCh:
		return true
	default:
		return false
	}
}

func (h *StreamHandle) EndedC() <-chan struct{} {
	return h.endedCh
}
