package sudp

import (
	"sync/atomic"
)

func newTestReusable[T any](data T, callToFree *atomic.Uint64) reusable[T] {
	return reusable[T]{
		data: data,
		free: func() {
			callToFree.Add(1)
		},
	}
}
