// Copyright (c) nano Author. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package nano

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"strings"
	"time"
	"io"
	"bytes"
	"bufio"
	"net/http/httptest"
)

type ServeType int

const (
	TCP       ServeType = iota
	WebSocket
	Hybrid
)

func listen(addr string, t ServeType, opts ...Option) {
	for _, opt := range opts {
		opt(handler.options)
	}

	hbdEncode()
	startupComponents()

	// create global ticker instance, timer precision could be customized
	// by SetTimerPrecision
	globalTicker = time.NewTicker(timerPrecision)

	// startup logic dispatcher
	go handler.dispatch()

	go func() {
		switch t {
		case TCP:
			listenAndServe(addr)
		case WebSocket:
			listenAndServeWS(addr)
		case Hybrid:
			listenAndServeHybrid(addr)
		default:
			logger.Fatal(fmt.Sprintf("unknown serve type %d, listen at %s", t, addr))
		}
	}()

	logger.Println(fmt.Sprintf("starting application %s, listen at %s", app.name, addr))
	sg := make(chan os.Signal)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL)

	// stop server
	select {
	case <-env.die:
		logger.Println("The app will shutdown in a few seconds")
	case s := <-sg:
		logger.Println("got signal", s)
	}

	logger.Println("server is stopping...")

	// shutdown all components registered by application, that
	// call by reverse order against register
	shutdownComponents()
}

// Enable current server accept connection
func listenAndServe(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatal(err.Error())
	}

	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Println(err.Error())
			continue
		}

		go handler.handle(conn)
	}
}

func listenAndServeWS(addr string) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     env.checkOrigin,
	}

	http.HandleFunc("/"+strings.TrimPrefix(env.wsPath, "/"), func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Println(fmt.Sprintf("Upgrade failure, URI=%s, Error=%s", r.RequestURI, err.Error()))
			return
		}

		handler.handleWS(conn)
	})

	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Fatal(err.Error())
	}
}

func listenAndServeHybrid(addr string) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     env.checkOrigin,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatal(err.Error())
	}

	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Println(err.Error())
			continue
		}

		go func() {
			m := make([]byte, 3)
			remain := 3

			for {
				n, err := conn.Read(m)
				if err != nil {
					logger.Println(err.Error())
					conn.Close()
					return
				}

				remain -= n
				if remain == 0 {
					break
				}
			}

			// WebSocket upgrade: MUST be GET, MUST be HTTP1.1 or greater
			if string(m) == "GET" {
				r, err := http.ReadRequest(bufio.NewReader(io.MultiReader(bytes.NewReader(m), conn)))
				if err != nil {
					logger.Println(err.Error())
					conn.Close()
					return
				}

				rc := httptest.NewRecorder()
				rw, err := newExtResponseWriter(rc, conn)
				if err != nil {
					logger.Println(err.Error())
					conn.Close()
					return
				}

				wsc, err := upgrader.Upgrade(rw, r, nil)
				if err != nil {
					logger.Println(err.Error())
					conn.Close()
					return
				}

				// actual response
				_, err = io.Copy(conn, rc.Result().Body)
				if err != nil {
					logger.Println(err.Error())
					conn.Close()
					return
				}

				handler.handleWS(wsc)
			} else {
				handler.handle(newExtConn(m, conn))
			}
		}()
	}
}
