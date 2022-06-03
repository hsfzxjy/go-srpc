package main

import (
	"errors"
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
		for i := 0; i < n; i++ {
			s.PushValue(i)
			s.Logf("Log from Server: i=%d\n", i)
		}
		return errors.New("example error")
	}, s, nil)
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
	// enumerate the result
	for x := range h.C() {
		log.Printf("recieve value from remote: %+v\n", x)
	}
	// check potential returned error
	if err := h.Result(); err != nil {
		log.Printf("remote returns error: %+v", err)
	}

	listener.Close()
}
