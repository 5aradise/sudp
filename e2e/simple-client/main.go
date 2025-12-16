package main

import (
	"log"
	"os"
	"time"

	"github.com/5aradise/sudp"
)

func main() {
	saddr, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok {
		panic("SERVER_ADDRESS is not set")
	}

	conn, err := sudp.Dial("udp", saddr)
	if err != nil {
		log.Fatalln("dial error:", err)
	}
	addr := conn.RemoteAddr()
	if addr == nil {
		addr = conn.LocalAddr()
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Fatalln("unexpected close error:", err)
		}

		log.Println("disconnected from", addr)
	}()

	log.Println("connected to", addr)

	buf := make([]byte, 1024)
	for i, msg := range []string{
		"Hello",
		"Hi, how are you?",
		"Is anyone there?",
		"This is a test message.",
		"Can you hear me?",
		"Everything seems to work fine.",
		"Let me know if you got this.",
		"Sending another message just in case.",
		"Thanks for your help!",
		"Goodbye!",
	} {
		_, err = conn.Write([]byte(msg))
		if err != nil {
			log.Fatalln("write error:", err)
		}

		n, err := conn.Read(buf)
		if err != nil {
			log.Fatalln("read error:", err)
		}

		received := string(buf[:n])
		if received == msg {
			log.Println("successfuly sended and received msg", i)
		} else {
			log.Fatalln("unexpected response:", received, "expected:", msg)
		}

		time.Sleep(time.Second)
	}

	log.Println("all messages sended and received successfuly")
}
