# go-srpc

A package that implements streaming RPC in Go, with highlights as:

- **No Dependancy** `srpc` merely extends the built-in `net/rpc`.
- **Ease of Use** `srpc` supports pushing versatile events to the client, including ordinary values or logging entries.
- **Server-side Timeout Control** Server is able to disconnect a stream after client being silent for a certain while.
- **Client-side Cancellation** Client is able to cancel an ongoing stream.

An example for quick glance:

```go
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
```

## Usage

### Server-side

To define a streaming method that would continuously push events to the client, one should use the following snippet

```go
import "github.com/hsfzxjy/srpc"

func (*Foo) Bar(arg ArgType, s *srpc.Session) error {
	return srpc.S(func() error {
		// your code goes here

        // push arbitary values to the client
        s.PushValue(42)
        s.PushValue("Hello world")

        // log something at the client-side
        s.Logf("Log from server! The arg is %v", arg)

        // wait for a client-side cancellation
        <-s.Canceled()

        // return an optional error to the client
        return nil
	}, s, nil)
```

The third argument of `srpc.S` can be used to configure the current session

```go
srpc.S(func() error { ... }, s, &SessionConfig {
    // The capacity of pushing channel.
    // PushValue() or Logf() will block if client does not recieve in-time.
    // Default: 10
    BufferCapacity: 0,
    // Timeout control for lazy clients.
    // If PushValue() or Logf() block for duration `ClientTimeout`,
    // they panic and abort the whole stream.
    // Default: 10 * time.Second
    ClientTimeout: 1 * time.Second,
    // Keep the session alive up to duration `KeepAlive` after it finished,
    // so that client is able to recieve remaining events.
    // Default: 10 * time.Second
    KeepAlive: 1 * time.Second,
})
```

### Client

To call a streaming method at remote, one should firstly wraps an existing `rpc.Client`, for example

```go
cl, _ := rpc.DialHTTP("tcp", "localhost:1234")
cli := srpc.WrapClient(cl)
```

`cli.CallStream()` allows you to invoke a remote streaming method

```go
h, err := cli.CallStream("Foo.Bar", arg)
```

`err` would be non-nil if the method does not exist. With `h`, you may recieve values, perform cancellation or inspect potential errors from remote

```go
// recieve values
<- h.C()
for x := h.C() {
    println(x)
}

// cancel the stream and no consequent events will be pushed
h.Cancel()

// inspect potential error
var err error = h.Err
// inspect potential panic
var p any = h.Panic
// or simply use h.Result()
// if remote panics, h.Result() will panic with the same value
err := h.Result()
```

Check [examples/](./examples/) for more concrete examples.

# [License](./LICENSE)

The Apache License Version 2.0
