package nano

import (
	"net"
	"bytes"
	"io"
)

type extendConn struct {
	extBytes *bytes.Reader
	net.Conn
}

func (c *extendConn) Read(b []byte) (n int, err error) {
	if c.extBytes != nil {
		n, err = io.MultiReader(c.extBytes, c.Conn).Read(b)
		if c.extBytes.Len() == 0 {
			c.extBytes = nil
		}
		return
	}
	return c.Conn.Read(b)
}

func newExtConn(eb []byte, conn net.Conn) *extendConn {
	if conn == nil {
		return nil
	}
	if eb != nil {
		return &extendConn{extBytes: bytes.NewReader(eb), Conn: conn}
	}
	return &extendConn{nil, conn}
}
