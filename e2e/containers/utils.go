package e2e

import (
	"crypto/rand"
	"log"
	"math"
	"os"
	"strconv"
	"time"
)

type Ping [2]time.Time

func (p Ping) Duration() time.Duration {
	return p[1].Sub(p[0])
}

type request struct {
	V []byte
}

func newRequest(size int) request {
	req := request{
		V: make([]byte, size),
	}
	req.Regenerate()
	return req
}

func (r *request) Regenerate() {
	n, err := rand.Read(r.V)
	if err != nil {
		panic(err)
	}
	if n != len(r.V) {
		panic("crypto/rand: short read")
	}
}

func ClientConfig() (saddr string, request request, requestCount int) {
	saddr, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok {
		panic("SERVER_ADDRESS is not set")
	}

	srsize, ok := os.LookupEnv("REQUEST_SIZE")
	if ok {
		rsize, err := strconv.Atoi(srsize)
		if err != nil {
			panic(err)
		}
		request = newRequest(rsize)
	}

	srcount, ok := os.LookupEnv("REQUEST_COUNT")
	if ok {
		var err error
		requestCount, err = strconv.Atoi(srcount)
		if err != nil {
			panic(err)
		}
	}

	return
}

func ServerConfig() (addr string, clientsCount int) {
	addr, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok {
		panic("SERVER_ADDRESS is not set")
	}

	sclients, ok := os.LookupEnv("CLIENTS_COUNT")
	if !ok {
		panic("CLIENTS_COUNT is not set")
	}
	var err error
	clientsCount, err = strconv.Atoi(sclients)
	if err != nil {
		panic(err)
	}

	return
}

func LogHighLowAvg(pings []Ping) {
	var (
		high = time.Duration(math.MinInt)
		low  = time.Duration(math.MaxInt)
		avg  = time.Duration(0)
	)

	for _, ping := range pings {
		dif := ping.Duration()
		high = max(high, dif)
		low = min(low, dif)
		avg += dif
	}
	avg /= time.Duration(len(pings))

	log.Printf("high: %s, low: %s, avg: %s", high, low, avg)
}
