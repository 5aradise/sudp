package sudp

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBufQueue_Read(t *testing.T) {
	t.Run("Simple message", func(t *testing.T) {
		assert := assert.New(t)
		q := newBufQueue()

		q.write([]byte("Hello from server!"))
		buf := make([]byte, 1024)
		n, err := q.read(buf)
		assert.NoError(err)

		assert.Equal([]byte("Hello from server!"), buf[:n])
	})

	t.Run("Queue order", func(t *testing.T) {
		assert := assert.New(t)
		q := newBufQueue()

		q.write([]byte{1, 2, 3})
		q.write([]byte{4, 5, 6})
		q.write([]byte{7, 8, 9})
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
	})

	t.Run("Buffering messages", func(t *testing.T) {
		assert := assert.New(t)
		q := newBufQueue()

		q.write([]byte(strings.Repeat("A", 420) + strings.Repeat("B", 69)))
		bufA := make([]byte, 420)
		nA, err := q.read(bufA)
		assert.NoError(err)
		bufB := make([]byte, 420)
		nB, err := q.read(bufB)
		assert.NoError(err)

		assert.Equal(strings.Repeat("A", 420), string(bufA[:nA]))
		assert.Equal(strings.Repeat("B", 69), string(bufB[:nB]))
	})

	t.Run("One data stream", func(t *testing.T) {
		assert := assert.New(t)
		q := newBufQueue()

		q.write([]byte(strings.Repeat("A", 420)))
		q.write([]byte(strings.Repeat("B", 69)))
		buf := make([]byte, 1024)
		n, err := q.read(buf)
		assert.NoError(err)

		assert.Equal(strings.Repeat("A", 420)+strings.Repeat("B", 69), string(buf[:n]))
	})

	t.Run("When empty will wait", func(t *testing.T) {
		assert := assert.New(t)
		q := newBufQueue()

		go func() {
			time.Sleep(time.Second)
			q.write([]byte{1, 2, 3})
		}()

		buf := make([]byte, 1024)
		n, err := q.read(buf)
		assert.NoError(err)

		assert.Equal([]byte{1, 2, 3}, buf[:n])
	})

	t.Run("When has message will not wait", func(t *testing.T) {
		assert := assert.New(t)
		q := newBufQueue()

		go func() {
			q.write([]byte{1, 2, 3})
			time.Sleep(time.Second)
			q.write([]byte{4, 5, 6})
		}()

		buf := make([]byte, 1024)
		n, err := q.read(buf)
		assert.NoError(err)

		assert.Equal([]byte{1, 2, 3}, buf[:n])
	})
}

func TestBufPacketReader_Close(t *testing.T) {
	t.Run("After close can be possibly to read buffered data", func(t *testing.T) {
		assert := assert.New(t)
		r := newBufQueue()

		r.write([]byte{1, 2, 3})
		r.write([]byte{4, 5, 6})
		r.close(errors.New(""))
		data1 := make([]byte, 4)
		n1, _ := r.read(data1)
		data2 := make([]byte, 4)
		n2, _ := r.read(data2)

		assert.Equal([]byte{1, 2, 3, 4}, data1[:n1])
		assert.Equal([]byte{5, 6}, data2[:n2])
	})

	t.Run("After close should return same error", func(t *testing.T) {
		assert := assert.New(t)
		r := newBufQueue()

		target := errors.New("test")
		r.write([]byte{1, 2, 3})
		r.write([]byte{4, 5, 6})
		r.close(target)
		data1 := make([]byte, 4)
		_, err1 := r.read(data1)
		data2 := make([]byte, 4)
		_, err2 := r.read(data2)

		assert.ErrorIs(err1, target)
		assert.ErrorIs(err2, target)
	})

	t.Run("After close write should panic (like a channel)", func(t *testing.T) {
		assert := assert.New(t)
		r := newBufQueue()
		r.write([]byte{1, 2, 3})

		r.close(errors.New(""))

		assert.Panics(func() {
			r.write([]byte{4, 5, 6})
		})
	})

	t.Run("After close close should panic (like a channel)", func(t *testing.T) {
		assert := assert.New(t)
		r := newBufQueue()
		r.write([]byte{1, 2, 3})

		r.close(errors.New(""))

		assert.Panics(func() {
			r.close(errors.New(""))
		})
	})
}
