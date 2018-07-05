package nano

import (
	"net/http/httptest"
	"net"
	"bufio"
	"errors"
)

var (
	errConnIsNil = errors.New("extendResponseWriter: conn is nil")
)

type extendResponseWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (rw *extendResponseWriter) Hijack() (rwc net.Conn, bufrw *bufio.ReadWriter, err error) {
	if rw.conn == nil {
		return nil, nil, errConnIsNil
	}

	rwc = rw.conn
	bufrw = bufio.NewReadWriter(bufio.NewReader(rw.conn), bufio.NewWriter(rw.conn))
	return
}

func newExtResponseWriter(rc *httptest.ResponseRecorder, conn net.Conn) (rw *extendResponseWriter, err error) {
	if conn == nil {
		return nil, errConnIsNil
	}

	return &extendResponseWriter{ResponseRecorder: rc, conn: conn}, nil
}
