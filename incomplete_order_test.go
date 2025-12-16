package sudp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncompleteOrder(t *testing.T) {
	pack := func(num uint32) packet {
		return packet{
			header: header{
				number: num,
			},
			data: []byte{byte(num)},
		}
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
		assert.Equal([]byte{0}, p)
	}
	// readed: [0]							incomplete: [2, 3, 6, 7]

	expectedPacketNum := byte(1)
	for p := range io.append(pack(1)) {
		assert.Equal([]byte{expectedPacketNum}, p)
		expectedPacketNum++
	}
	assert.EqualValues(4, expectedPacketNum, "packet with this number not in queue")
	// readed: [0, 1, 2, 3]					incomplete: [6, 7]

	for p := range io.append(pack(4)) {
		assert.Equal([]byte{4}, p)
	}
	// readed: [0, 1, 2, 3, 4]				incomplete: [6, 7]

	for p := range io.append(pack(8)) {
		assert.Fail("unordered packet", p)
	}
	// readed: [0, 1, 2, 3, 4]				incomplete: [6, 7, 8]

	expectedPacketNum = 5
	for p := range io.append(pack(5)) {
		assert.Equal([]byte{expectedPacketNum}, p)
		expectedPacketNum++
	}
	assert.EqualValues(9, expectedPacketNum, "packet with this number not in queue")
	// readed: [0, 1, 2, 3, 4, 5, 6, 7, 8]	incomplete: []
}

func TestIncompleteOrder_CommandPacket(t *testing.T) {
	pack := func(num uint32) packet {
		return packet{
			header: header{
				number: num,
			},
			data: []byte{byte(num)},
		}
	}
	commandPack := func(num uint32) packet {
		return packet{
			header: header{
				isCommand: true,
				number:    num,
			},
			data: []byte{byte(num)},
		}
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
		actualPacketNums = append(actualPacketNums, p...)
	}
	assert.Equal(expectedPacketNums, actualPacketNums)
}

func TestBinaryInsert(t *testing.T) {
	pack := func(num uint32) packet {
		return packet{
			header: header{
				number: num,
			},
		}
	}
	assert := assert.New(t)

	res := binaryInsert([]packet{pack(1), pack(3)}, pack(2))
	assert.Equal([]packet{pack(1), pack(2), pack(3)}, res)

	res = binaryInsert([]packet{pack(1), pack(3), pack(7)}, pack(2))
	assert.Equal([]packet{pack(1), pack(2), pack(3), pack(7)}, res)

	res = binaryInsert([]packet{pack(1), pack(3), pack(7)}, pack(4))
	assert.Equal([]packet{pack(1), pack(3), pack(4), pack(7)}, res)

	res = binaryInsert([]packet{pack(1), pack(3), pack(7)}, pack(5))
	assert.Equal([]packet{pack(1), pack(3), pack(5), pack(7)}, res)

	res = binaryInsert([]packet{pack(1), pack(3), pack(7)}, pack(8))
	assert.Equal([]packet{pack(1), pack(3), pack(7), pack(8)}, res)
}
