package sudp

import (
	"iter"
	"slices"
)

type incompleteOrder struct {
	nextToRead uint32
	incomplete []reusable[packet]
}

func (o *incompleteOrder) append(p reusable[packet]) (completed iter.Seq[reusable[[]byte]]) {
	if p.data.number != o.nextToRead {
		o.incomplete = binaryInsert(o.incomplete, p)
		return func(yield func(reusable[[]byte]) bool) {}
	}

	return func(yield func(reusable[[]byte]) bool) {
		o.nextToRead++
		if !yieldDataPacket(yield, p) {
			return
		}

		lri := -1 // last readed index
		for i, p := range o.incomplete {
			if p.data.number != o.nextToRead {
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

func yieldDataPacket(yield func(reusable[[]byte]) bool, p reusable[packet]) bool {
	if p.data.isCommand {
		p.free()
		return true
	}

	return yield(reusable[[]byte]{
		data: p.data.data,
		free: p.free,
	})
}

func binaryInsert(ps []reusable[packet], p reusable[packet]) []reusable[packet] {
	i, _ := slices.BinarySearchFunc(ps, p, func(a, b reusable[packet]) int { return int(a.data.number) - int(b.data.number) })
	return slices.Insert(ps, i, p)
}
