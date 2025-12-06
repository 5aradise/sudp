package sudp

import (
	"errors"
	"fmt"
	"io"
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

	readCh := make(chan []byte, readConnChSize)
	readErr := new(error)
	go func(ch chan []byte, err *error, src io.Reader) {
		for {
			buf := make([]byte, maxPacketSize)
			n, rerr := src.Read(buf)
			if err != nil {
				*err = rerr
				close(ch)
				return
			}
			ch <- buf[:n]
		}
	}(readCh, readErr, src)
	conn := newConn(readCh, readErr, src, nil)
	return &dconn{
		conn:    conn,
		addrSrc: src,
	}, nil
}

type dconn struct {
	*conn
	addrSrc net.Conn
}

func (c *dconn) LocalAddr() net.Addr {
	return c.addrSrc.LocalAddr()
}

func (c *dconn) RemoteAddr() net.Addr {
	return c.addrSrc.RemoteAddr()
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
