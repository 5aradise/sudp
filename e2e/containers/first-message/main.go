package main

import (
	"bytes"
	"log"
	"time"

	"github.com/5aradise/sudp"

	e2e "github.com/5aradise/sudp/e2e/containers"
)

func main() {
	saddr, req, _ := e2e.ClientConfig()

	start := time.Now()
	conn, err := sudp.Dial("udp", saddr)
	if err != nil {
		log.Fatalln("dial error:", err)
	}

	_, err = conn.Write(req.V)
	if err != nil {
		log.Fatalln("write error:", err)
	}

	res := make([]byte, len(req.V))
	n, err := conn.Read(res)
	if err != nil {
		log.Fatalln("read error:", err)
	}
	end := time.Now()

	if !bytes.Equal(req.V, res[:n]) {
		log.Fatalln("unexpected message:", string(res[:n]))
	}

	err = conn.Close()
	if err != nil {
		log.Fatalln("unexpected close error:", err)
	}

	log.Println("time:", e2e.Ping{start, end}.Duration())
}
