package sudp

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConn_Close(t *testing.T) {
	t.Run("Error from close (internal event) should overwrite error from external event", func(t *testing.T) {
		assert := assert.New(t)
		in := make(chan []byte)
		close(in)
		rerr := errors.New("read err")
		inerr := &rerr
		out := errWriter{errors.New("write err")}
		conn := newConn(in, inerr, out, nil)

		err := conn.Close()
		assert.NoError(err)
		buf := make([]byte, 1024)
		wn, werr := conn.Write(buf)
		assert.Zero(wn)
		rn, rerr := conn.Read(buf)
		assert.Zero(rn)

		assert.ErrorIs(werr, net.ErrClosed)
		assert.ErrorIs(rerr, net.ErrClosed)
	})

	t.Run("Out close call only once", func(t *testing.T) {
		assert := assert.New(t)
		var outCloseCount int
		outClose := func() error {
			outCloseCount++
			return nil
		}
		conn := newConn(nil, nil, nil, outClose)

		err := conn.Close()
		assert.NoError(err)
		err = conn.Close()
		assert.NoError(err)

		assert.Equal(1, outCloseCount)
	})
}

type errWriter struct {
	err error
}

func (w errWriter) Write(b []byte) (int, error) {
	return 0, w.err
}
