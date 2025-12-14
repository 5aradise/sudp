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
		conns:    make(map[netip.AddrPort]chan<- []byte),
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
	conns          map[netip.AddrPort]chan<- []byte
}

func (l *listener) Accept() (net.Conn, error) {
	newConn := <-l.newConns

	if l.newConnsClosed.Load() {
		if rerr := l.rerr.Load(); rerr != nil {
			return nil, rerr.(error)
		}
		return nil, errCloseFuncCalled
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
	readErr := new(error)
	for {
		buf := make([]byte, maxPacketSize)
		n, addr, err := l.src.ReadFromUDPAddrPort(buf)
		if err != nil {
			l.rerr.Store(fmt.Errorf("failed to read from main connection: %w", err))
			l.newConnsClosed.Store(true)

			*readErr = err
			l.connsMu.Lock()
			for _, conn := range l.conns {
				close(conn)
			}
			clear(l.conns)
			l.connsMu.Unlock()

			l.src.Close()
			return
		}

		l.connsMu.RLock()
		connCh, ok := l.conns[addr]
		if !ok {
			newCh := l.lockedNewConn(addr, readErr)
			if newCh == nil {
				continue
			}

			connCh = newCh
		}
		connCh <- buf[:n]
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

type connWriter struct {
	addr netip.AddrPort
	srv  *net.UDPConn
}

func (w connWriter) Write(b []byte) (int, error) {
	return w.srv.WriteToUDPAddrPort(b, w.addr)
}

func (l *listener) onConnCLose(addr netip.AddrPort) func() error {
	return func() error {
		l.connsMu.Lock()
		l.conns[addr] <- nil
		delete(l.conns, addr)
		l.connsMu.Unlock()
		return l.tryCloseSrc()
	}
}

func (l *listener) lockedNewConn(addr netip.AddrPort, readErr *error) chan<- []byte {
	readCh := make(chan []byte, readConnChSize)

	conn := newConn(readCh, readErr, connWriter{addr: addr, srv: l.src}, l.onConnCLose(addr))

	if l.newConnsClosed.Load() {
		close(l.newConns)
		return nil
	}
	l.conns[addr] = readCh
	l.newConns <- &lconn{conn: conn, addr: addr}

	return readCh
}

type lconn struct {
	*conn
	addr netip.AddrPort
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
