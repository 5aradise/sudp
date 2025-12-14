package sudp

import (
	"slices"
)

type number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// represents range of numbers (with inclusive bounds):
//
// [0] - first number in range
//
// [1] - last number in range
type rng[T number] [2]T

func (r rng[T]) in(n T) bool {
	return r[0] <= n && n <= r[1]
}

// if range already contains n, does nothing and returns false
func rangesTryAppend[T number](rs []rng[T], n T) ([]rng[T], bool) {
	if len(rs) == 0 { // ( n )
		return append(rs, rng[T]{n, n}), true
	}

	bsri := -1 // bsr = biggest smaller range
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i][1] < n {
			bsri = i
			break
		}
	}
	if bsri != len(rs)-1 { // ( n [next] ... ) or ( ... [bsr] n [next] ... )
		next := rs[bsri+1]
		if next.in(n) {
			return rs, false
		}

		if next[0]-1 == n { // ( n[next] ... ) or ( ... [bsr] n[next] ... )
			if bsri != -1 { // ( ... [bsr] n[next] ... )
				bsr := rs[bsri]
				if bsr[1]+1 == n { // ( ... [bsr]n[next] ... )
					return slices.Replace(rs, bsri, bsri+2, rng[T]{bsr[0], next[1]}), true
				}
			}
			rs[bsri+1] = rng[T]{n, next[1]}
			return rs, true
		}
	}
	if bsri != -1 { // ( ... [bsr] n ... )
		bsr := rs[bsri]
		if bsr[1]+1 == n { // ( ... [bsr]n ... )
			rs[bsri] = rng[T]{bsr[0], n}
			return rs, true
		}
	}
	// ( n [next] ... ) or ( ... [bsr] n [next] ... ) or ( ... [next] n)
	return slices.Insert(rs, bsri+1, rng[T]{n, n}), true
}
