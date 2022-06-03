package srpc

import (
	"encoding/gob"
	"fmt"
)

type streamEventType int32

func (typ streamEventType) IsTerminal() bool {
	return typ == seDone || typ == seError || typ == sePanic
}

const (
	seValue streamEventType = iota
	seError
	seLog
	seDone
	sePanic
)

type StreamEvent struct {
	Typ  streamEventType
	Data any
}

type panicInfo struct {
	Data  any
	Stack []byte
}

func (pi *panicInfo) Error() string {
	return fmt.Sprintf("%v", pi)
}

func (pi *panicInfo) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('+') {
			f.Write([]byte(fmt.Sprintf("%+v\n=====> REMOTE STACK START <=====\n", pi.Data)))
			f.Write(pi.Stack)
			f.Write([]byte("=====> REMOTE STACK END <=====\n"))
			return
		}
		fallthrough
	default:
		f.Write([]byte(fmt.Sprintf("%+v", pi.Data)))
	}
}

func init() {
	var _ fmt.Formatter = &panicInfo{}
	gob.Register(panicInfo{})
}
