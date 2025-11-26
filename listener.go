package sudp

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

const newConnBufSize = 128

func Listen(network, address string) (net.Listener, error) {
	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}
	conn, err := net.ListenUDP(network, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	l := &listener{
		src:      conn,
		newConns: make(chan net.Conn, newConnBufSize),
		conns:    make(map[netip.AddrPort]writeOnlyQueue),
	}
	go l.listen()
	return l, nil
}

type listener struct {
	src            *net.UDPConn
	rerr           atomic.Value
	newConnsClosed atomic.Bool
	newConns       chan net.Conn
	connsMu        sync.RWMutex
	conns          map[netip.AddrPort]writeOnlyQueue
}

func (l *listener) Accept() (net.Conn, error) {
	newConn := <-l.newConns

	if l.newConnsClosed.Load() {
		if rerr := l.rerr.Load(); rerr != nil {
			return nil, rerr.(error)
		}
		return nil, fmt.Errorf("%w: close function called", net.ErrClosed)
	}
	return newConn, nil
}

func (l *listener) Close() error {
	l.newConnsClosed.Store(true)
	return l.tryCloseSrc()
}

func (l *listener) Addr() net.Addr {
	return l.src.LocalAddr()
}

func (l *listener) listen() {
	for {
		buf := make([]byte, maxPacketSize)
		n, addr, err := l.src.ReadFromUDPAddrPort(buf)
		if err != nil {
			l.rerr.Store(fmt.Errorf("failed to read from main connection: %w", err))
			l.newConnsClosed.Store(true)
			return // TODO: maybe another logic
		}

		p, _ := decodePacket(buf[:n])

		l.connsMu.RLock()
		connBuf, ok := l.conns[addr]
		if !ok {
			newBuf := l.lockedNewConn(addr)
			if newBuf == nil {
				continue
			}

			connBuf = newBuf
		}
		connBuf.write(p.data)
		l.connsMu.RUnlock()
	}
}

func (l *listener) tryCloseSrc() error {
	if !l.newConnsClosed.Load() {
		return nil
	}

	l.connsMu.RLock()
	activeConns := len(l.conns)
	l.connsMu.RUnlock()
	if activeConns != 0 {
		return nil
	}

	err := l.src.Close()
	if err != nil {
		if errors.Is(err, net.ErrClosed) { // if src was closed, close quitely
			return nil
		}
		return fmt.Errorf("failed to close main connection: %w", err)
	}
	return nil
}

func (l *listener) lockedNewConn(addr netip.AddrPort) writeOnlyQueue {
	bufr := newBufQueue()
	conn := &lconn{
		addr: addr,
		r:    bufr,
		w:    l.src,
		outClose: func() error {
			l.connsMu.Lock()
			delete(l.conns, addr)
			l.connsMu.Unlock()
			return l.tryCloseSrc()
		},
	}

	if l.newConnsClosed.Load() {
		close(l.newConns)
		return nil
	}
	l.conns[addr] = bufr
	l.newConns <- conn

	return bufr
}

type lconn struct {
	addr     netip.AddrPort
	r        *bufQueue
	w        udpWriter
	werr     atomic.Value
	outClose func() error
}

type udpWriter interface {
	WriteToUDPAddrPort(b []byte, addr netip.AddrPort) (int, error)
}

func (l *lconn) Read(b []byte) (int, error) {
	return l.r.read(b)
}

func (c *lconn) Write(b []byte) (int, error) {
	if werr := c.werr.Load(); werr != nil {
		return 0, werr.(error)
	}

	var n int
	ps, _ := dataIntoPackets(0, b)
	msg := make([]byte, maxPacketSize)
	for _, p := range ps {
		packetSize, err := p.encode(msg)
		if err != nil {
			panic(err)
		}
		written, err := c.w.WriteToUDPAddrPort(msg[:packetSize], c.addr)
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

func (c *lconn) Close() error {
	if c.werr.Load() != nil { // already closed
		return nil
	}
	err := c.outClose()

	rwerr := fmt.Errorf("%w: close function called", net.ErrClosed)
	c.werr.Store(rwerr)
	c.r.close(rwerr)

	return err
}

func (c *lconn) LocalAddr() net.Addr {
	if !c.addr.Addr().IsPrivate() {
		return nil
	}

	return net.UDPAddrFromAddrPort(c.addr)
}

func (c *lconn) RemoteAddr() net.Addr {
	if c.addr.Addr().IsPrivate() {
		return nil
	}

	return net.UDPAddrFromAddrPort(c.addr)
}

// For now SUDP doesn't support deadline
func (c *lconn) SetDeadline(t time.Time) error {
	return fmt.Errorf("%w: temporarily not implemented", errors.ErrUnsupported)
}

// For now SUDP doesn't support deadline
func (c *lconn) SetReadDeadline(t time.Time) error {
	return fmt.Errorf("%w: temporarily not implemented", errors.ErrUnsupported)
}

// For now SUDP doesn't support deadline
func (c *lconn) SetWriteDeadline(t time.Time) error {
	return fmt.Errorf("%w: temporarily not implemented", errors.ErrUnsupported)
}
