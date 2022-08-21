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
		s.PushValue(n + 1)

		s.Logv("slow job started")
		// emulate a slow job
		job := time.NewTimer(5 * time.Second)
		defer job.Stop()
		select {
		case <-job.C:
		case <-s.EndC():
			return nil
		}
		s.Logv("slow job ended")

		s.PushValue(n + 2)
		return nil
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
	log.Printf("recieve value from remote: %+v\n", <-h.C())
	// cancel the remote stream
	go h.Cancel()
	<- h.C()
	h.Cancel()

	// check potential returned error
	if err := h.Result(); err != nil {
		log.Printf("remote returns error: %+v", err)
	}

	listener.Close()
}
