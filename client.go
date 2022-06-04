package srpc

import (
	"net/rpc"
)

type Client struct {
	*rpc.Client
}

func (c *Client) CallStream(name string, args any) (h *streamHandle, err error) {
	handle := streamHandle{
		sid:    0,
		ch:     make(chan any),
		client: c,
		state:  0,
	}
	var sess Session
	err = c.Call(name, args, &sess)
	if err != nil {
		return
	}
	handle.sid = sess.Sid
	h = &handle

	return
}

func WrapClient(c *rpc.Client) *Client {
	return &Client{Client: c}
}
