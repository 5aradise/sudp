package sudp

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestListener(t *testing.T) {
	t.Run("Shouldn't accept tcp", func(t *testing.T) {
		assert := assert.New(t)

		_, err := Listen("tcp", "127.0.0.1:0")

		assert.Error(err)
	})
}

func TestListener_Accept(t *testing.T) {
	t.Run("Concurrent connections", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)
		var connCount atomic.Int64
		var wg sync.WaitGroup

		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 10, 100*time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{4, 5, 6}, 10, 100*time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{7, 8, 9}, 10, 100*time.Millisecond)
		for range 3 {
			conn, err := l.Accept()
			assert.NoError(err)

			wg.Go(func() {
				defer func() {
					assert.NoError(conn.Close())
				}()

				buf := make([]byte, 1024)
				_, err = conn.Read(buf)
				assert.NoError(err)

				connCount.Add(1)

				for range 9 {
					_, err = conn.Read(buf)
					assert.NoError(err)
				}
			})
		}

		time.Sleep(300 * time.Millisecond)
		assert.Equal(3, int(connCount.Load()))
		wg.Wait()
	})

	t.Run("Should drop connections if newConns buffer is full", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)
		var idleClients sync.WaitGroup

		for range newConnsCap {
			idleClients.Go(func() {
				periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 1, 0)
			})
		}
		idleClients.Wait()

		// all clients sended messages, so buffer should be full

		conn, err := Dial("udp", l.Addr().String())
		assert.NoError(err)
		defer func() {
			assert.NoError(conn.Close())
		}()

		_, err = conn.Write([]byte{1, 2, 3})
		assert.NoError(err)

		_, err = conn.Read(make([]byte, 1024))
		assert.ErrorIs(err, net.ErrClosed)
	})
}

func TestListener_Close(t *testing.T) {
	t.Run("Shouldn't accept connections after close", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)

		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 10, time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{4, 5, 6}, 10, time.Millisecond)
		conn, err := l.Accept()
		assert.NoError(err)
		buf := make([]byte, 1024)
		for range 10 {
			_, err = conn.Read(buf)
			assert.NoError(err)
		}
		assert.NoError(conn.Close())
		err = l.Close()
		assert.NoError(err)
		conn, err = l.Accept()

		assert.Nil(conn)
		assert.ErrorIs(err, net.ErrClosed)
	})

	t.Run("Should be able to communicate with existing connections after close", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)
		var lstnClosedMu sync.Mutex
		lstnClosedCond := sync.NewCond(&lstnClosedMu)
		var lstnClosed bool
		var wg sync.WaitGroup

		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 10, 100*time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{4, 5, 6}, 10, 100*time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{7, 8, 9}, 10, 100*time.Millisecond)
		for range 3 {
			conn, err := l.Accept()
			assert.NoError(err)
			wg.Go(func() {
				defer func() {
					assert.NoError(conn.Close())
				}()
				buf := make([]byte, 1024)
				_, err := conn.Read(buf)
				assert.NoError(err)

				lstnClosedMu.Lock()
				for !lstnClosed {
					lstnClosedCond.Wait()
				}
				lstnClosedMu.Unlock()

				_, err = conn.Write([]byte{10, 11, 12})
				assert.NoError(err)
				for range 9 {
					_, err = conn.Read(buf)
					assert.NoError(err)
				}
			})
		}
		err = l.Close()
		assert.NoError(err)
		lstnClosedMu.Lock()
		lstnClosed = true
		lstnClosedMu.Unlock()
		lstnClosedCond.Broadcast()
		wg.Wait()
	})

	t.Run("Twice close shouldn't return error", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)

		err = l.Close()
		assert.NoError(err)
		err = l.Close()

		assert.NoError(err)
	})
}

func TestListenerConn_Close(t *testing.T) {
	t.Run("Can't read after close", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)

		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 2, time.Minute)
		conn, err := l.Accept()
		assert.NoError(err)
		buf := make([]byte, 1024)
		_, err = conn.Read(buf)
		assert.NoError(err)
		err = conn.Close()
		assert.NoError(err)
		n, err := conn.Read(buf)

		assert.Zero(n)
		assert.ErrorIs(err, net.ErrClosed)
	})

	t.Run("Can't write after close", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)

		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 2, time.Minute)
		conn, err := l.Accept()
		assert.NoError(err)
		_, err = conn.Write([]byte{1, 2, 3})
		assert.NoError(err)
		err = conn.Close()
		assert.NoError(err)
		n, err := conn.Write([]byte{4, 5, 6})

		assert.Zero(n)
		assert.ErrorIs(err, net.ErrClosed)
	})

	t.Run("Twice close shouldn't return error", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)

		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 2, time.Minute)
		conn, err := l.Accept()
		assert.NoError(err)
		err = conn.Close()
		assert.NoError(err)
		err = conn.Close()

		assert.NoError(err)
	})

	t.Run("When listener is closed and then all connections are closed main connection should be closed", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)
		var wg sync.WaitGroup
		var lstnClosedMu sync.Mutex
		lstnClosedCond := sync.NewCond(&lstnClosedMu)
		var lstnClosed bool

		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 1, time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 1, time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 1, time.Millisecond)
		for range 3 {
			conn, err := l.Accept()
			assert.NoError(err)
			wg.Go(func() {
				buf := make([]byte, 1024)
				_, err := conn.Read(buf)
				assert.NoError(err)
				lstnClosedMu.Lock()
				for !lstnClosed {
					lstnClosedCond.Wait()
				}
				lstnClosedMu.Unlock()
				assert.NoError(conn.Close())
			})
		}
		assert.NoError(l.Close())
		lstnClosedMu.Lock()
		lstnClosed = true
		lstnClosedMu.Unlock()
		lstnClosedCond.Broadcast()
		wg.Wait()

		err = l.(*listener).src.Close()
		assert.ErrorIs(err, net.ErrClosed)
	})

	t.Run("When all connections are closed and then listener is closed main connection should be closed", func(t *testing.T) {
		assert := assert.New(t)
		l, err := Listen("udp", "127.0.0.1:0")
		assert.NoError(err)
		var wg sync.WaitGroup

		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 1, time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 1, time.Millisecond)
		go periodicalClientMsg(assert, l.Addr().String(), []byte{1, 2, 3}, 1, time.Millisecond)
		for range 3 {
			conn, err := l.Accept()
			assert.NoError(err)
			wg.Go(func() {
				buf := make([]byte, 1024)
				_, err := conn.Read(buf)
				assert.NoError(err)
				assert.NoError(conn.Close())
			})
		}
		wg.Wait()
		assert.NoError(l.Close())

		err = l.(*listener).src.Close()
		assert.ErrorIs(err, net.ErrClosed)
	})
}

func TestListenerConn_SetDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := lconn{}

		err := conn.SetDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}

func TestListenerConn_SetReadDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := lconn{}

		err := conn.SetReadDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}

func TestListenerConn_SetWriteDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := lconn{}

		err := conn.SetWriteDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}

func periodicalClientMsg(assert *assert.Assertions,
	srvAddr string, msg []byte, tickCount int, tick time.Duration) {
	conn, err := Dial("udp", srvAddr)
	assert.NoError(err)
	defer func() {
		assert.NoError(conn.Close())
	}()

	for range tickCount {
		_, err := conn.Write(msg)
		assert.NoError(err)

		time.Sleep(tick)
	}
}
