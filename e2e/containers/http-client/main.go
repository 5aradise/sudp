package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/5aradise/sudp"

	e2e "github.com/5aradise/sudp/e2e/containers"
)

func main() {
	saddr, _, _ := e2e.ClientConfig()

	cl := &http.Client{
		Transport: SudpDefaultTransport,
	}

	req, err := http.NewRequest(http.MethodPost,
		"http://"+saddr+"/users/5aradise",
		strings.NewReader("Creator of sudp"))
	if err != nil {
		log.Fatalln("new request error:", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Age", "19")

	res, err := cl.Do(req)
	if err != nil {
		log.Fatalln("request error:", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		log.Fatalln("unexpected status code:", res.StatusCode)
	}

	resData, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalln("read error:", err)
	}
	if string(resData) != "Name: 5aradise\nAge: 19\nDescription: Creator of sudp" {
		log.Fatalln("unexpected response:", string(resData))
	}

	log.Println("success")
}

// default tcp transport from stdlib but with sudp dialer
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
