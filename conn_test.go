package sudp

import (
	"bytes"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConn_ReceivedPackets(t *testing.T) {
	t.Run("Should send received packets after short timer if no new data in small window", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		in := make(chan reusable[[]byte], 2)
		inerr := new(error)
		out := &testPacketBuffer{t: t}
		_ = newConn(in, inerr, out, nil)

		msg := make([]byte, 1024)
		n, err := dataPacket(0, []byte("Hello")).encode(msg)
		assert.NoError(err)
		in <- newTestReusable(msg[:n], &freeCalls)
		msg = make([]byte, 1024)
		n, err = dataPacket(1, []byte("World")).encode(msg)
		assert.NoError(err)
		in <- newTestReusable(msg[:n], &freeCalls)

		time.Sleep(rShortTime)

		time.Sleep(deliveryDelay / 2)

		ps := out.Packets()
		assert.GreaterOrEqual(len(ps), 1)
		received := testDecodeReceivedPackets(t, ps[0])
		assert.Equal([]rng[uint32]{{0, 1}}, received)
		assert.EqualValues(0, freeCalls.Load(), "should keep all packets")
	})

	t.Run("Should send received packets after long timer if new data in small window", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		in := make(chan reusable[[]byte], 2)
		inerr := new(error)
		out := &testPacketBuffer{t: t}
		smallWindow := rShortTime / 2
		smallWindowPackets := int(rLongTime / smallWindow)
		restToTime := rLongTime - smallWindow*time.Duration(smallWindowPackets)
		_ = newConn(in, inerr, out, nil)

		for i := range smallWindowPackets {
			msg := make([]byte, 1024)
			n, err := dataPacket(uint32(i+69), []byte("Hello")).encode(msg)
			assert.NoError(err)
			in <- newTestReusable(msg[:n], &freeCalls)
			time.Sleep(smallWindow)
		}
		time.Sleep(restToTime)

		time.Sleep(deliveryDelay / 2)

		ps := out.Packets()
		assert.GreaterOrEqual(len(ps), 1)
		received := testDecodeReceivedPackets(t, ps[0])
		assert.Equal([]rng[uint32]{{69, uint32(69 + smallWindowPackets - 1)}}, received)
		assert.EqualValues(0, freeCalls.Load(), "should keep all packets")
	})

	t.Run("Should instantly send received packets if receive received packet", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		in := make(chan reusable[[]byte], 2)
		inerr := new(error)
		out := &testPacketBuffer{t: t}
		_ = newConn(in, inerr, out, nil)

		msg := make([]byte, 1024)
		n, err := dataPacket(0, []byte("Hello")).encode(msg)
		assert.NoError(err)
		in <- newTestReusable(msg[:n], &freeCalls)
		msg = make([]byte, 1024)
		n, err = dataPacket(1, []byte("World")).encode(msg)
		assert.NoError(err)
		in <- newTestReusable(msg[:n], &freeCalls)
		msg = make([]byte, 1024)
		n, err = dataPacket(1, []byte("World")).encode(msg)
		assert.NoError(err)
		in <- newTestReusable(msg[:n], &freeCalls)
		msg = make([]byte, 1024)
		n, err = dataPacket(2, []byte("World")).encode(msg)
		assert.NoError(err)
		in <- newTestReusable(msg[:n], &freeCalls)

		time.Sleep(deliveryDelay / 2)

		ps := out.Packets()
		assert.GreaterOrEqual(len(ps), 1)
		received := testDecodeReceivedPackets(t, ps[0])
		assert.Equal([]rng[uint32]{{0, 1}}, received)
		assert.EqualValues(1, freeCalls.Load(), "should free received packet")
	})
}

func TestConn_Close(t *testing.T) {
	t.Run("Error from close (internal event) should overwrite error from external event", func(t *testing.T) {
		assert := assert.New(t)
		in := make(chan reusable[[]byte])
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
		in := make(chan reusable[[]byte])
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
