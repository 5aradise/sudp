package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/5aradise/sudp"
)

func main() {
	addr, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok {
		panic("SERVER_ADDRESS is not set")
	}

	mux := http.NewServeMux()
	setupRoutes(mux)

	log.Println("HTTP server listening on", addr)

	err := listenAndServe(addr, mux)
	if err != nil {
		log.Println(err)
	}
}

func setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /{name}", func(w http.ResponseWriter, r *http.Request) {
		log.Println("new request:", r.RemoteAddr)

		name := r.PathValue("name")
		age := r.Header.Get("Age")
		desc, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		_, err = fmt.Fprintf(w, "Name: %s\nAge: %s\nDescription: %s\n", name, age, desc)
		if err != nil {
			log.Println("writing response error:", err)
		}
	})
}

func listenAndServe(addr string, handler http.Handler) error {
	ln, err := sudp.Listen("udp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	err = http.Serve(ln, handler)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}
