package sudp

import (
	"sync/atomic"
)

const bufQueueSize = 64

// newBufPacketReader returns buffered asyncronous bytes queue
func newBufQueue() *bufQueue {
	return &bufQueue{
		ch: make(chan []byte, bufQueueSize),
	}
}

type bufQueue struct {
	ch  chan []byte
	err atomic.Value // to stop reading error should be set and channel closed
	buf []byte
}

// asyncronous [io.Reader] implementation, returns error from [bufQueue.close] call
func (r *bufQueue) read(b []byte) (int, error) {
	var data []byte
	if len(r.buf) != 0 {
		data = r.buf
	} else {
		data = <-r.ch // block reading if no buffered data
	}
	copied := copy(b, data)
	n := copied

	for n < len(b) && len(r.ch) > 0 {
		data = <-r.ch
		copied = copy(b[n:], data)
		n += copied
	}

	r.buf = data[copied:]
	err, _ := r.err.Load().(error)
	return n, err
}

func (r *bufQueue) write(p []byte) {
	r.ch <- p
}

func (r *bufQueue) close(err error) {
	close(r.ch)
	r.err.Store(err)
}
