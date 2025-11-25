package sudp

import (
	"errors"
	"fmt"
	"net"
	"time"
)

func Dial(network, address string) (net.Conn, error) {
	serverAddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	src, err := net.DialUDP(network, nil, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	conn := &dconn{
		src:  src,
		bufr: newBufQueue(),
	}
	go conn.read()
	return conn, nil
}

type dconn struct {
	src  net.Conn
	bufr *bufQueue
}

func (c *dconn) Read(b []byte) (int, error) {
	return c.bufr.read(b)
}

func (c *dconn) Write(b []byte) (int, error) {
	return c.src.Write(b)
}

func (c *dconn) Close() error {
	return c.src.Close()
}

func (c *dconn) LocalAddr() net.Addr {
	return c.src.LocalAddr()
}

func (c *dconn) RemoteAddr() net.Addr {
	return c.src.RemoteAddr()
}

// For now SUDP doesn't support deadline
func (c *dconn) SetDeadline(t time.Time) error {
	return fmt.Errorf("%w: temporarily not implemented", errors.ErrUnsupported)
}

// For now SUDP doesn't support deadline
func (c *dconn) SetReadDeadline(t time.Time) error {
	return fmt.Errorf("%w: temporarily not implemented", errors.ErrUnsupported)
}

// For now SUDP doesn't support deadline
func (c *dconn) SetWriteDeadline(t time.Time) error {
	return fmt.Errorf("%w: temporarily not implemented", errors.ErrUnsupported)
}

func (c *dconn) read() {
	for {
		buf := make([]byte, 1024)
		n, err := c.src.Read(buf)
		if err != nil {
			c.bufr.close(fmt.Errorf("failed to read from connection: %w", err))
			return // TODO: maybe another logic
		}
		c.bufr.write(buf[:n])
	}
}
