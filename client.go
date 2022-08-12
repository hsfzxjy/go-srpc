package srpc

import (
	"net/rpc"
)

type Client struct {
	*rpc.Client
}

func (c *Client) CallStream(name string, args any) (h *StreamHandle, err error) {
	h = newStreamHandle(c)
	var sess Session
	err = c.Call(name, args, &sess)
	if err != nil {
		return
	}
	h.sid = sess.Sid

	return
}

func WrapClient(c *rpc.Client) *Client {
	return &Client{Client: c}
}
