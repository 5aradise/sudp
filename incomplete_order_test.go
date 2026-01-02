package sudp

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncompleteOrder(t *testing.T) {
	var freeCalls atomic.Uint64
	pack := func(num uint32) reusable[packet] {
		return newTestReusable(packet{
			header: header{
				number: num,
			},
			data: []byte{byte(num)},
		}, &freeCalls)
	}
	assert := assert.New(t)

	io := incompleteOrder{}
	for p := range io.append(pack(2)) {
		assert.Fail("unordered packet", p)
	}
	for p := range io.append(pack(3)) {
		assert.Fail("unordered packet", p)
	}
	for p := range io.append(pack(6)) {
		assert.Fail("unordered packet", p)
	}
	for p := range io.append(pack(7)) {
		assert.Fail("unordered packet", p)
	}
	// readed: []							incomplete: [2, 3, 6, 7]

	for p := range io.append(pack(0)) {
		assert.Equal([]byte{0}, p.data)
		p.free()
	}
	// readed: [0]							incomplete: [2, 3, 6, 7]

	expectedPacketNum := byte(1)
	for p := range io.append(pack(1)) {
		assert.Equal([]byte{expectedPacketNum}, p.data)
		p.free()
		expectedPacketNum++
	}
	assert.EqualValues(4, expectedPacketNum, "packet with this number not in queue")
	// readed: [0, 1, 2, 3]					incomplete: [6, 7]

	for p := range io.append(pack(4)) {
		assert.Equal([]byte{4}, p.data)
		p.free()
	}
	// readed: [0, 1, 2, 3, 4]				incomplete: [6, 7]

	for p := range io.append(pack(8)) {
		assert.Fail("unordered packet", p.data)
		p.free()
	}
	// readed: [0, 1, 2, 3, 4]				incomplete: [6, 7, 8]

	expectedPacketNum = 5
	for p := range io.append(pack(5)) {
		assert.Equal([]byte{expectedPacketNum}, p.data)
		p.free()
		expectedPacketNum++
	}
	assert.EqualValues(9, expectedPacketNum, "packet with this number not in queue")
	// readed: [0, 1, 2, 3, 4, 5, 6, 7, 8]	incomplete: []

	assert.EqualValues(9, freeCalls.Load())
}

func TestIncompleteOrder_CommandPacket(t *testing.T) {
	var freeCalls atomic.Uint64
	pack := func(num uint32) reusable[packet] {
		return newTestReusable(packet{
			header: header{
				number: num,
			},
			data: []byte{byte(num)},
		}, &freeCalls)
	}
	commandPack := func(num uint32) reusable[packet] {
		return newTestReusable(packet{
			header: header{
				isCommand: true,
				number:    num,
			},
			data: []byte{byte(num)},
		}, &freeCalls)
	}
	assert := assert.New(t)

	io := incompleteOrder{}
	io.append(pack(1))
	io.append(pack(2))
	io.append(commandPack(3))
	io.append(pack(4))
	io.append(pack(5))
	io.append(commandPack(6))
	io.append(commandPack(7))

	expectedPacketNums := []byte{1, 2, 4, 5}
	var actualPacketNums []byte
	for p := range io.append(commandPack(0)) {
		actualPacketNums = append(actualPacketNums, p.data...)
		p.free()
	}
	assert.Equal(expectedPacketNums, actualPacketNums)
	assert.EqualValues(8, freeCalls.Load())
}

func TestBinaryInsert(t *testing.T) {
	pack := func(num uint32) reusable[packet] {
		return reusable[packet]{
			packet{
				header: header{
					number: num,
				},
			},
			nil,
		}
	}
	assert := assert.New(t)

	res := binaryInsert([]reusable[packet]{pack(1), pack(3)}, pack(2))
	assert.Equal([]reusable[packet]{pack(1), pack(2), pack(3)}, res)

	res = binaryInsert([]reusable[packet]{pack(1), pack(3), pack(7)}, pack(2))
	assert.Equal([]reusable[packet]{pack(1), pack(2), pack(3), pack(7)}, res)

	res = binaryInsert([]reusable[packet]{pack(1), pack(3), pack(7)}, pack(4))
	assert.Equal([]reusable[packet]{pack(1), pack(3), pack(4), pack(7)}, res)

	res = binaryInsert([]reusable[packet]{pack(1), pack(3), pack(7)}, pack(5))
	assert.Equal([]reusable[packet]{pack(1), pack(3), pack(5), pack(7)}, res)

	res = binaryInsert([]reusable[packet]{pack(1), pack(3), pack(7)}, pack(8))
	assert.Equal([]reusable[packet]{pack(1), pack(3), pack(7), pack(8)}, res)
}
