package sudp

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConn_ReceivedPackets(t *testing.T) {
	t.Run("Should send received packets after short timer if no new data in small window", func(t *testing.T) {
		assert := assert.New(t)
		in := make(chan []byte, 2)
		inerr := new(error)
		out := &testPacketBuffer{t: t}
		_ = newConn(in, inerr, out, nil)

		msg := make([]byte, 1024)
		n, err := dataPacket(0, []byte("Hello")).encode(msg)
		assert.NoError(err)
		in <- msg[:n]
		msg = make([]byte, 1024)
		n, err = dataPacket(1, []byte("World")).encode(msg)
		assert.NoError(err)
		in <- msg[:n]

		time.Sleep(sShortTime)

		ps := out.Packets()
		assert.Len(ps, 1)
		received := testDecodeReceivedPackets(t, ps[0])
		assert.Equal([]rng[uint32]{{0, 1}}, received)
	})

	t.Run("Should send received packets after long timer if new data in small window", func(t *testing.T) {
		assert := assert.New(t)
		in := make(chan []byte, 2)
		inerr := new(error)
		out := &testPacketBuffer{t: t}
		_ = newConn(in, inerr, out, nil)

		for i := range 11 {
			msg := make([]byte, 1024)
			n, err := dataPacket(uint32(i+69), []byte("Hello")).encode(msg)
			assert.NoError(err)
			in <- msg[:n]
			time.Sleep(rLongTime / 11)
		}

		ps := out.Packets()
		assert.Len(ps, 1)
		received := testDecodeReceivedPackets(t, ps[0])
		assert.Equal([]rng[uint32]{{69, 79}}, received)
	})

	t.Run("Should send received packets if receive received packet", func(t *testing.T) {
		assert := assert.New(t)
		in := make(chan []byte, 2)
		inerr := new(error)
		out := &testPacketBuffer{t: t}
		_ = newConn(in, inerr, out, nil)

		msg := make([]byte, 1024)
		n, err := dataPacket(0, []byte("Hello")).encode(msg)
		assert.NoError(err)
		in <- msg[:n]
		msg = make([]byte, 1024)
		n, err = dataPacket(1, []byte("World")).encode(msg)
		assert.NoError(err)
		in <- msg[:n]
		msg = make([]byte, 1024)
		n, err = dataPacket(1, []byte("World")).encode(msg)
		assert.NoError(err)
		in <- msg[:n]
		msg = make([]byte, 1024)
		n, err = dataPacket(2, []byte("World")).encode(msg)
		assert.NoError(err)
		in <- msg[:n]

		time.Sleep(sShortTime)

		ps := out.Packets()
		assert.Len(ps, 2)
		received := testDecodeReceivedPackets(t, ps[0])
		assert.Equal([]rng[uint32]{{0, 1}}, received)
		received = testDecodeReceivedPackets(t, ps[1])
		assert.Equal([]rng[uint32]{{0, 2}}, received)
	})
}

func TestConn_Close(t *testing.T) {
	t.Run("Error from close (internal event) should overwrite error from external event", func(t *testing.T) {
		assert := assert.New(t)
		in := make(chan []byte)
		close(in)
		inRawErr := errors.New("read err")
		inerr := &inRawErr
		out := errWriter{errors.New("write err")}
		conn := newConn(in, inerr, out, nil)

		_ = conn.Close()
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
		in := make(chan []byte)
		inerr := new(error)
		out := bytes.NewBuffer(nil)
		var outCloseCount int
		outClose := func() error {
			outCloseCount++
			return nil
		}
		conn := newConn(in, inerr, out, outClose)

		err := conn.Close()
		assert.NoError(err)
		_ = conn.Close()

		assert.Equal(1, outCloseCount)
	})
}

type errWriter struct {
	err error
}

func (w errWriter) Write(b []byte) (int, error) {
	return 0, w.err
}
