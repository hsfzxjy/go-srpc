package srpc

import (
	"encoding/gob"
	"fmt"
	"reflect"
)

type dummyWriter int

func (*dummyWriter) Write(p []byte) (int, error) { return len(p), nil }

var dummyGobEncoder = gob.NewEncoder(new(dummyWriter))

type SrpcError struct {
	ErrorString string
	Typ         string
}

func (s *SrpcError) Error() string { return s.ErrorString }
func (s *SrpcError) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('+') {
			f.Write([]byte(fmt.Sprintf("(%s) %s", s.Typ, s.ErrorString)))
			return
		}
		fallthrough
	default:
		f.Write([]byte(s.ErrorString))
	}
}

func init() {
	gob.Register(&SrpcError{})
}

func transformError(obj error) error {
	if obj == nil {
		return nil
	}
	if err := dummyGobEncoder.Encode(obj); err == nil {
		return obj
	}
	return &SrpcError{
		ErrorString: obj.Error(),
		Typ:         fmt.Sprintf("%+v",reflect.TypeOf(obj)),
	}
}
