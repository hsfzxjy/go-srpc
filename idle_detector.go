package srpc

import (
	"time"
)

type idleDetectorEventType int

const (
	idEnter idleDetectorEventType = iota
	idLeave
	idQuit
	idTimer
)

type idleDetectorEvent struct {
	typ  idleDetectorEventType
	time time.Time
}

type idleDetector struct {
	q            chan idleDetectorEvent
	exited       chan struct{}
	counter      int32
	lastIdleTime time.Time
	duration     time.Duration
	notifier     func()
}

func newIdleDetector(duration time.Duration, notifier func()) *idleDetector {
	id := new(idleDetector)
	id.q = make(chan idleDetectorEvent, 1)
	id.q <- idleDetectorEvent{typ: idLeave}
	id.counter = 1
	id.exited = make(chan struct{})
	id.duration = duration
	id.notifier = notifier
	return id
}

func (id *idleDetector) push(typ idleDetectorEventType) {
	select {
	case <-id.exited:
	case id.q <- idleDetectorEvent{typ: typ}:
	}
}

func (id *idleDetector) loop() {
	defer func() {
		close(id.exited)
		id.notifier = nil
	}()

	if id.duration == 0 {
		return
	}

	for event := range id.q {
		switch event.typ {
		case idEnter:
			id.counter++
		case idLeave:
			id.counter--
			if id.counter == 0 {
				id.lastIdleTime = time.Now()
				timer := time.NewTimer(id.duration)
				go func() {
					t := <-timer.C
					select {
					case id.q <- idleDetectorEvent{idTimer, t}:
					case <-id.exited:
					}
				}()
			}
		case idQuit:
			return
		case idTimer:
			if id.counter != 0 ||
				event.time.Sub(id.lastIdleTime) <= id.duration {
				continue
			}
			id.notifier()
			return
		}
	}
}
