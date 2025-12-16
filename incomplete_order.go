package sudp

import (
	"iter"
	"slices"
)

type incompleteOrder struct {
	nextToRead uint32
	incomplete []packet
}

func (o *incompleteOrder) append(p packet) (completed iter.Seq[[]byte]) {
	if p.number != o.nextToRead {
		o.incomplete = binaryInsert(o.incomplete, p)
		return func(yield func([]byte) bool) {}
	}

	return func(yield func([]byte) bool) {
		o.nextToRead++
		if !yieldDataPacket(yield, p) {
			return
		}

		lri := -1 // last readed index
		for i, p := range o.incomplete {
			if p.number != o.nextToRead {
				lri = i - 1
				break
			}

			o.nextToRead++
			if !yieldDataPacket(yield, p) {
				break
			}
		}
		if lri != -1 {
			o.incomplete = slices.Delete(o.incomplete, 0, lri+1)
		}
	}
}

func yieldDataPacket(yield func([]byte) bool, p packet) bool {
	if p.isCommand {
		return true
	}

	return yield(p.data)
}

func binaryInsert(ps []packet, p packet) []packet {
	i, _ := slices.BinarySearchFunc(ps, p, func(a, b packet) int { return int(a.number) - int(b.number) })
	return slices.Insert(ps, i, p)
}
