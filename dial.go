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
	var n int
	ps, _ := dataIntoPackets(0, b)
	msg := make([]byte, maxPacketSize)
	for _, p := range ps {
		packetSize, err := p.encode(msg)
		if err != nil {
			panic(err)
		}
		written, err := c.src.Write(msg[:packetSize])
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
		buf := make([]byte, maxPacketSize)
		n, err := c.src.Read(buf)
		if err != nil {
			c.bufr.close(fmt.Errorf("failed to read from connection: %w", err))
			return // TODO: maybe another logic
		}

		p, _ := decodePacket(buf[:n])

		c.bufr.write(p.data)
	}
}
