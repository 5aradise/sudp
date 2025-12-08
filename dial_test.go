package sudp

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDial(t *testing.T) {
	t.Run("Shouldn't accept tcp", func(t *testing.T) {
		assert := assert.New(t)

		_, err := Dial("tcp", "127.0.0.1:8090")

		assert.Error(err)
	})
}

func TestDialConn_Close(t *testing.T) {
	t.Run("Can't read after close", func(t *testing.T) {
		assert := assert.New(t)
		saddr := periodicalServerMsg(assert, []byte{1, 2, 3}, 10, time.Millisecond)
		conn, err := Dial("udp", saddr)
		assert.NoError(err)
		_, err = conn.Write([]byte("Hello"))
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
		saddr := periodicalServerMsg(assert, []byte{1, 2, 3}, 10, time.Millisecond)
		conn, err := Dial("udp", saddr)
		assert.NoError(err)
		_, err = conn.Write([]byte("Hello"))
		assert.NoError(err)

		_, err = conn.Write([]byte{1, 2, 3})
		assert.NoError(err)
		err = conn.Close()
		assert.NoError(err)
		n, err := conn.Write([]byte{4, 5, 6})

		assert.Zero(n)
		assert.ErrorIs(err, net.ErrClosed)
	})
}

func TestDialConn_SetDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := dconn{}

		err := conn.SetDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}

func TestDialConn_SetReadDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := dconn{}

		err := conn.SetReadDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}

func TestDialConn_SetWriteDeadline(t *testing.T) {
	t.Run("Should be unsupported", func(t *testing.T) {
		assert := assert.New(t)
		conn := dconn{}

		err := conn.SetWriteDeadline(time.Now())

		assert.ErrorIs(err, errors.ErrUnsupported)
	})
}

func periodicalServerMsg(assert *assert.Assertions, msg []byte, tickCount int, tick time.Duration) string {
	l, err := Listen("udp", "127.0.0.1:0")
	assert.NoError(err)

	go func() {
		defer func() {
			assert.NoError(l.Close())
		}()

		for {
			conn, err := l.Accept()
			assert.NoError(err)
			go func() {
				for range tickCount {
					_, err = conn.Write(msg)
					assert.NoError(err)

					time.Sleep(tick)
				}
			}()
		}
	}()

	return l.Addr().String()
}
