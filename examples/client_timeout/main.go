package main

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"time"

	"github.com/hsfzxjy/go-srpc"
)

type Foo int

func (*Foo) Bar(n int, s *srpc.Session) error {
	return srpc.S(func() error {
		<-s.EndedC()
		return s.EndCause
	}, s, &srpc.SessionConfig{
		BufferCapacity: 0,
		ClientTimeout:  100 * time.Millisecond,
	})
}

func main() {
	// start RPC server
	rpc.Register(new(Foo))
	rpc.HandleHTTP()
	listener, _ := net.Listen("tcp", ":1234")
	go http.Serve(listener, nil)

	time.Sleep(time.Millisecond * 10)

	// prepare RPC client
	cl, _ := rpc.DialHTTP("tcp", "localhost:1234")
	cli := srpc.WrapClient(cl)

	// invoke remote stream function
	h, _ := cli.CallStream("Foo.Bar", 6)
	// emulate a lazy client
	time.Sleep(200 * time.Millisecond)
	<-h.C()

	// check potential returned error
	if err := h.Result(); err != nil {
		log.Printf("remote returns error: %+v", err)
	}

	listener.Close()
}
