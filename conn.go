package sudp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
)

const readConnChSize = 64

var (
	ErrPacketCorrupted = errors.New("packet corrupted while writing")
	errCloseFuncCalled = fmt.Errorf("%w: close function called", net.ErrClosed)
)

// inerr - reading error from main connection (should be set before closing r channel)
func newConn(in <-chan []byte, inerr *error, out io.Writer, onClose func() error) *conn {
	if onClose == nil {
		onClose = func() error { return nil }
	}
	c := &conn{
		toRead:  newBufQueue(),
		toWrite: make(chan []byte),
		out: struct {
			r     <-chan []byte
			rerr  *error
			w     io.Writer
			close func() error
		}{in, inerr, out, onClose},
	}
	go c.run()
	return c
}

// An error in reading and writing may occur for two reasons related to the connection:
// 1. It is closed (both directions must be notified at once)
// 2. Problems with the external main connection
// (the error will appear when attempting to interact with the external connection,
// so there is no need to notify the other direction)

type conn struct {
	toRead   *bufQueue
	toWrite  chan []byte
	closeErr atomic.Value
	out      struct {
		r    <-chan []byte
		rerr *error // will be set after close r channel

		w io.Writer

		close func() error
	}
}

func (c *conn) run() {
	for {
		data := <-c.out.r
		if data == nil {
			if clErr := c.closeErr.Load(); clErr != nil {
				c.toRead.close(clErr.(error))
			} else {
				c.toRead.close(fmt.Errorf("failed to read from main connection: %w", *c.out.rerr))
			}
			return
		}

		p, err := decodePacket(data)
		if err != nil {
			// TODO
		}
		c.toRead.write(p.data)
	}
}

func (l *conn) Read(b []byte) (int, error) {
	return l.toRead.read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	if clErr := c.closeErr.Load(); clErr != nil {
		return 0, clErr.(error)
	}

	var n int
	ps, _ := dataIntoPackets(0, b)
	msg := make([]byte, maxPacketSize)
	for _, p := range ps {
		packetSize, err := p.encode(msg)
		if err != nil { // should never happen
			panic(err)
		}
		written, err := c.out.w.Write(msg[:packetSize])
		if err != nil {
			return 0, fmt.Errorf("failed to write to main connection: %w", err)
		}
		if written != packetSize {
			return 0, ErrPacketCorrupted
		}
		n += len(p.data)
	}
	return n, nil
}

func (c *conn) Close() error {
	return c.closeWithError(errCloseFuncCalled)
}

func (c *conn) closeWithError(err error) error {
	prevErr := c.closeErr.Load()

	c.closeErr.Store(err)

	if prevErr == nil { // first call
		return c.out.close()
	}
	return nil
}
