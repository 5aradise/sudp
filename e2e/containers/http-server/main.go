package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/5aradise/sudp"

	e2e "github.com/5aradise/sudp/e2e/containers"
)

func main() {
	addr, reqCount := e2e.ServerConfig()

	mux := http.NewServeMux()
	h := newCloseAfterRequestsHandler(reqCount)
	mux.HandleFunc("POST /users/{name}", h.createUser)

	s := http.Server{
		Handler: mux,
	}

	go func() {
		log.Println("HTTP server listening")

		ln, err := sudp.Listen("udp", addr)
		if err != nil {
			log.Println("listen error:", err)
		}

		err = s.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalln("serve error:", err)
		}
	}()

	h.reqs.Wait()

	log.Println("all requests handled")
	err := s.Shutdown(context.Background())
	if err != nil {
		log.Fatalln("shutdown error:", err)
	}
	log.Println("server shutdown")
}

func newCloseAfterRequestsHandler(reqCount int) *closeAfterRequestsHandler {
	wg := &sync.WaitGroup{}
	wg.Add(reqCount)
	return &closeAfterRequestsHandler{
		reqs: wg,
	}
}

type closeAfterRequestsHandler struct {
	reqs *sync.WaitGroup
}

func (h *closeAfterRequestsHandler) createUser(w http.ResponseWriter, r *http.Request) {
	defer h.reqs.Done()

	name := r.PathValue("name")
	age := r.Header.Get("Age")
	desc, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = fmt.Fprintf(w, "Name: %s\nAge: %s\nDescription: %s", name, age, desc)
	if err != nil {
		log.Println("writing response error:", err)
	}
}
