package main

import (
	"errors"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/5aradise/sudp"
)

var (
	connErrsMu sync.Mutex
	connErrs   []string
	wg         sync.WaitGroup
)

func main() {
	saddr, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok {
		panic("SERVER_ADDRESS is not set")
	}
	clients := math.MaxInt
	sclients, ok := os.LookupEnv("CLIENTS_COUNT")
	if ok {
		var err error
		clients, err = strconv.Atoi(sclients)
		if err != nil {
			panic(err)
		}
	}

	ln, err := sudp.Listen("udp", saddr)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	defer func() {
		err := ln.Close()
		if err != nil {
			log.Fatalln("close listener error:", err)
		}
	}()

	log.Println("TCP server listening on", ln.Addr())

	var connected int
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalln("accept error:", err)
			return
		}
		connected++

		wg.Go(func() { handleConn(conn) })

		if connected == clients {
			break
		}
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
			connErrs = append(connErrs, "close connection: "+err.Error())
			connErrsMu.Unlock()
		}
	}()

	log.Println("client connected:", addr)

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil && !errors.Is(err, net.ErrClosed) {
			connErrsMu.Lock()
			connErrs = append(connErrs, "read: "+err.Error())
			connErrsMu.Unlock()
			break
		}

		_, err = conn.Write(buf[:n])
		if err != nil && !errors.Is(err, net.ErrClosed) {
			connErrsMu.Lock()
			connErrs = append(connErrs, "write: "+err.Error())
			connErrsMu.Unlock()
			break
		}
	}

	log.Println("client disconnected:", addr)
}
