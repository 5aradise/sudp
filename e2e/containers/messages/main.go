package main

import (
	"bytes"
	"io"
	"log"
	"time"

	"github.com/5aradise/sudp"

	e2e "github.com/5aradise/sudp/e2e/containers"
)

func main() {
	saddr, req, reqCount := e2e.ClientConfig()

	conn, err := sudp.Dial("udp", saddr)
	if err != nil {
		log.Fatalln("dial error:", err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Fatalln("unexpected close error:", err)
		}

		log.Println("disconnected")
	}()

	pingTimeDifs := make([]e2e.Ping, 0, reqCount)

	res := make([]byte, len(req.V))
	for range reqCount {
		start := time.Now()

		_, err := conn.Write(req.V)
		if err != nil {
			log.Fatalln("write error:", err)
		}

		_, err = io.ReadFull(conn, res)
		if err != nil {
			log.Fatalln("read error:", err)
		}

		end := time.Now()

		if !bytes.Equal(req.V, res) {
			log.Fatalln("unexpected message:", string(res))
		}

		req.Regenerate()

		pingTimeDifs = append(pingTimeDifs, e2e.Ping{start, end})
	}

	e2e.LogHighLowAvg(pingTimeDifs)
}
