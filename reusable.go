package sudp

import (
	"sync"
)

var packetBufPool = sync.Pool{New: func() any { return make([]byte, maxPacketSize) }}

func getPacketBuf() reusable[[]byte] {
	v := packetBufPool.Get().([]byte)
	return reusable[[]byte]{
		data: v,
		free: func() { packetBufPool.Put(v) },
	}
}

type reusable[T any] struct {
	data T
	free func()
}
