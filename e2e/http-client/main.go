package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/5aradise/sudp"
)

func main() {
	addr, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok {
		panic("SERVER_ADDRESS is not set")
	}

	cl := &http.Client{
		Transport: SudpDefaultTransport,
	}

	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/Van", strings.NewReader("My name is Van"))
	if err != nil {
		log.Println("new request error:", err)
		return
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Age", "20")

	res, err := cl.Do(req)
	if err != nil {
		log.Println("request error:", err)
		return
	}
	defer res.Body.Close()

	resData, err := io.ReadAll(res.Body)
	if err != nil {
		log.Println("read error:", err)
		return
	}

	log.Println("received:\n", string(resData))
}

var SudpDefaultTransport http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return sudp.Dial("udp", addr)
	},
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}
