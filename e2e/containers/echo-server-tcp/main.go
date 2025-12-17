package main

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"

	e2e "github.com/5aradise/sudp/e2e/containers"
)

const (
	bufSize = 4096
)

var (
	connErrsMu sync.Mutex
	connErrs   []string
	wg         sync.WaitGroup
)

func main() {
	addr, clientCount := e2e.ServerConfig()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	defer func() {
		err := ln.Close()
		if err != nil {
			log.Fatalln("close listener error:", err)
		}

		log.Println("listner closed")
	}()

	log.Println("server listening on", ln.Addr())

	for range clientCount {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalln("accept error:", err)
			return
		}

		wg.Go(func() { handleConn(conn) })
	}

	wg.Wait()

	log.Println("all connections closed")

	if len(connErrs) > 0 {
		log.Fatalln("connection errors:", connErrs)
	}
}

func handleConn(conn net.Conn) {
	addr := conn.RemoteAddr()
	if addr == nil {
		addr = conn.LocalAddr()
	}

	defer func() {
		err := conn.Close()
		if err != nil && !errors.Is(err, net.ErrClosed) {
			connErrsMu.Lock()
			connErrs = append(connErrs, "close: "+err.Error())
			connErrsMu.Unlock()
		}

		log.Println("client disconnected:", addr)
	}()

	log.Println("client connected:", addr)

	buf := make([]byte, bufSize)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
				connErrsMu.Lock()
				connErrs = append(connErrs, "read: "+err.Error())
				connErrsMu.Unlock()
			}
			break
		}

		_, err = conn.Write(buf[:n])
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				connErrsMu.Lock()
				connErrs = append(connErrs, "read: "+err.Error())
				connErrsMu.Unlock()
			}
			break
		}
	}
}
