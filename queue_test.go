package sudp

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBufQueue_Read(t *testing.T) {
	t.Run("Simple message", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		q := newBufQueue()

		q.write(newTestReusable([]byte("Hello from server!"), &freeCalls))
		buf := make([]byte, 1024)
		n, err := q.read(buf)
		assert.NoError(err)

		assert.Equal([]byte("Hello from server!"), buf[:n])
		assert.EqualValues(1, freeCalls.Load())
	})

	t.Run("Queue order", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		q := newBufQueue()

		q.write(newTestReusable([]byte{1, 2, 3}, &freeCalls))
		q.write(newTestReusable([]byte{4, 5, 6}, &freeCalls))
		q.write(newTestReusable([]byte{7, 8, 9}, &freeCalls))
		buf1 := make([]byte, 3)
		n1, err := q.read(buf1)
		assert.NoError(err)
		buf2 := make([]byte, 3)
		n2, err := q.read(buf2)
		assert.NoError(err)
		buf3 := make([]byte, 3)
		n3, err := q.read(buf3)
		assert.NoError(err)

		assert.Equal([]byte{1, 2, 3}, buf1[:n1])
		assert.Equal([]byte{4, 5, 6}, buf2[:n2])
		assert.Equal([]byte{7, 8, 9}, buf3[:n3])
		assert.EqualValues(3, freeCalls.Load())
	})

	t.Run("Buffering messages", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		q := newBufQueue()

		q.write(newTestReusable([]byte(strings.Repeat("A", 420)+strings.Repeat("B", 69)), &freeCalls))
		bufA := make([]byte, 420)
		nA, err := q.read(bufA)
		assert.NoError(err)
		bufB := make([]byte, 420)
		nB, err := q.read(bufB)
		assert.NoError(err)

		assert.Equal(strings.Repeat("A", 420), string(bufA[:nA]))
		assert.Equal(strings.Repeat("B", 69), string(bufB[:nB]))
		assert.EqualValues(1, freeCalls.Load())
	})

	t.Run("One data stream", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		q := newBufQueue()

		q.write(newTestReusable([]byte(strings.Repeat("A", 420)), &freeCalls))
		q.write(newTestReusable([]byte(strings.Repeat("B", 69)), &freeCalls))
		buf := make([]byte, 1024)
		n, err := q.read(buf)
		assert.NoError(err)

		assert.Equal(strings.Repeat("A", 420)+strings.Repeat("B", 69), string(buf[:n]))
		assert.EqualValues(2, freeCalls.Load())
	})

	t.Run("When empty will wait", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		q := newBufQueue()

		go func() {
			time.Sleep(time.Second)
			q.write(newTestReusable([]byte{1, 2, 3}, &freeCalls))
		}()

		buf := make([]byte, 1024)
		n, err := q.read(buf)
		assert.NoError(err)

		assert.Equal([]byte{1, 2, 3}, buf[:n])
		assert.EqualValues(1, freeCalls.Load())
	})

	t.Run("When has message will not wait", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		q := newBufQueue()

		go func() {
			q.write(newTestReusable([]byte{1, 2, 3}, &freeCalls))
			time.Sleep(time.Second)
			q.write(newTestReusable([]byte{4, 5, 6}, &freeCalls))
		}()

		buf := make([]byte, 1024)
		n, err := q.read(buf)
		assert.NoError(err)

		assert.Equal([]byte{1, 2, 3}, buf[:n])
		assert.EqualValues(1, freeCalls.Load())
	})
}

func TestBufPacketReader_Close(t *testing.T) {
	t.Run("After close can be possibly to read buffered data", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		r := newBufQueue()

		r.write(newTestReusable([]byte{1, 2, 3}, &freeCalls))
		r.write(newTestReusable([]byte{4, 5, 6}, &freeCalls))
		r.close(errors.New(""))
		data1 := make([]byte, 4)
		n1, _ := r.read(data1)
		data2 := make([]byte, 4)
		n2, _ := r.read(data2)

		assert.Equal([]byte{1, 2, 3, 4}, data1[:n1])
		assert.Equal([]byte{5, 6}, data2[:n2])
		assert.EqualValues(2, freeCalls.Load())
	})

	t.Run("After close should return same error", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		r := newBufQueue()

		target := errors.New("test")
		r.write(newTestReusable([]byte{1, 2, 3}, &freeCalls))
		r.write(newTestReusable([]byte{4, 5, 6}, &freeCalls))
		r.close(target)
		data1 := make([]byte, 4)
		_, err1 := r.read(data1)
		data2 := make([]byte, 4)
		_, err2 := r.read(data2)

		assert.ErrorIs(err1, target)
		assert.ErrorIs(err2, target)
		assert.EqualValues(2, freeCalls.Load())
	})

	t.Run("After close write should panic (like a channel)", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		r := newBufQueue()
		r.write(newTestReusable([]byte{1, 2, 3}, &freeCalls))

		r.close(errors.New(""))

		assert.Panics(func() {
			r.write(newTestReusable([]byte{4, 5, 6}, &freeCalls))
		})
	})

	t.Run("After close close should panic (like a channel)", func(t *testing.T) {
		assert := assert.New(t)
		var freeCalls atomic.Uint64
		r := newBufQueue()
		r.write(newTestReusable([]byte{1, 2, 3}, &freeCalls))

		r.close(errors.New(""))

		assert.Panics(func() {
			r.close(errors.New(""))
		})
	})
}
