package sudp

import (
	"sync/atomic"
)

const defaultQueueSize = 1024

// newBufPacketReader returns buffered asyncronous bytes queue
func newBufQueue(size ...int) *bufQueue {
	s := defaultQueueSize
	if len(size) > 0 {
		s = size[0]
	}
	return &bufQueue{
		ch: make(chan reusable[[]byte], s),
	}
}

type bufQueue struct {
	ch  chan reusable[[]byte]
	err atomic.Value // to stop reading error should be set and channel closed
	buf reusable[[]byte]
}

// asyncronous [io.Reader] implementation, returns error from [bufQueue.close] call
func (r *bufQueue) read(b []byte) (int, error) {
	data := r.buf
	if len(data.data) == 0 {
		data = <-r.ch // block reading if no buffered data
	}
	copied := copy(b, data.data)
	n := copied

	for n < len(b) && len(r.ch) > 0 {
		data.free()
		data = <-r.ch
		copied = copy(b[n:], data.data)
		n += copied
	}

	if len(data.data[copied:]) > 0 {
		data = reusable[[]byte]{data.data[copied:], data.free}
	} else {
		if data.free != nil {
			data.free()
		}
		data = reusable[[]byte]{}
	}
	r.buf = data
	err, _ := r.err.Load().(error)
	return n, err
}

func (r *bufQueue) write(p reusable[[]byte]) {
	r.ch <- p
}

func (r *bufQueue) close(err error) {
	close(r.ch)
	r.err.Store(err)
}
