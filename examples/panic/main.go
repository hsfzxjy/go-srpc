package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"time"

	"github.com/hsfzxjy/go-srpc"
)

type Foo int

func (*Foo) Bar(n int, s *srpc.Session) error {
	return srpc.S(func() error {
		panic(errors.New("example panic"))
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
	<-h.C()

	// check panic
	fmt.Printf("remote panic: %+v\n", h.Panic)

	listener.Close()
}
