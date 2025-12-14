package sudp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoding(t *testing.T) {
	t.Run("Data", func(t *testing.T) {
		assert := assert.New(t)

		data := []byte("Hello from server!")
		target := packet{
			header{
				version:   1,
				isCommand: false,
				number:    69,
			},
			data,
		}
		buf := make([]byte, headerSize+len(data))
		n, err := target.encode(buf)
		assert.NoError(err)
		res, err := decodePacket(buf[:n])
		assert.NoError(err)

		assert.Equal(target, res)
	})

	t.Run("Command", func(t *testing.T) {
		assert := assert.New(t)

		data := []byte("Does not matter")
		target := packet{
			header{
				version:   1,
				isCommand: true,
				number:    420,
			},
			data,
		}
		buf := make([]byte, headerSize+len(data))
		n, err := target.encode(buf)
		assert.NoError(err)
		res, err := decodePacket(buf[:n])
		assert.NoError(err)

		assert.Equal(target, res)
	})

	t.Run("Can be only header", func(t *testing.T) {
		assert := assert.New(t)

		target := packet{
			header{
				version:   1,
				isCommand: true,
				number:    420,
			},
			nil,
		}
		buf := make([]byte, headerSize)
		n, err := target.encode(buf)
		assert.NoError(err)
		_, err = decodePacket(buf[:n])

		assert.NoError(err)
	})

	t.Run("Too small buffer", func(t *testing.T) {
		assert := assert.New(t)

		data := []byte("Hello from client!")
		target := packet{
			header{
				version:   1,
				isCommand: false,
				number:    89,
			},
			data,
		}
		buf := make([]byte, headerSize+len(data)-1)
		_, err := target.encode(buf)

		assert.ErrorIs(err, errTooSmallBuffer)
	})

	t.Run("Too small buffer", func(t *testing.T) {
		assert := assert.New(t)

		_, err := decodePacket([]byte{3, 3})

		assert.ErrorIs(err, errTooSmallPacket)
	})
}

func TestCommandPacket(t *testing.T) {
	t.Run("Close connection", func(t *testing.T) {
		assert := assert.New(t)

		p := closeConnectionPacket(69)
		tp, payload, err := commandPacketType(p)
		assert.NoError(err)

		assert.True(p.isCommand)
		assert.EqualValues(69, p.number)
		assert.Equal(commandCloseConn, tp)
		assert.Nil(payload)
	})

	t.Run("Received packets", func(t *testing.T) {
		assert := assert.New(t)

		p := receivedPacketsPacket(69, []rng[uint32]{{0, 3}, {5, 5}, {7, 8}, {11, 12}})
		tp, payload, err := commandPacketType(p)
		assert.NoError(err)
		recieved, err := decodeReceivedPackets(payload)
		assert.NoError(err)

		assert.True(p.isCommand)
		assert.EqualValues(69, p.number)
		assert.Equal(commandReceivedPackets, tp)
		assert.Equal([]rng[uint32]{{0, 3}, {5, 5}, {7, 8}, {11, 12}}, recieved)
	})

	t.Run("Invalid range format", func(t *testing.T) {
		assert := assert.New(t)

		_, err := decodeReceivedPackets([]byte{
			245, 3, 78, 95, 33, 104,
		})

		assert.ErrorIs(err, errInvalidRangeFormat)
	})

	t.Run("Received a lot of packets", func(t *testing.T) {
		assert := assert.New(t)

		p := receivedPacketsPacket(333, make([]rng[uint32], 293))
		tp, payload, err := commandPacketType(p)
		assert.NoError(err)
		recieved, err := decodeReceivedPackets(payload)
		assert.NoError(err)

		assert.True(p.isCommand)
		assert.EqualValues(333, p.number)
		assert.Equal(commandReceivedPackets, tp)
		assert.Equal(make([]rng[uint32], 293), recieved)
	})

	t.Run("Should panic when too many received packets", func(t *testing.T) {
		assert := assert.New(t)

		assert.Panics(func() {
			_ = receivedPacketsPacket(333, make([]rng[uint32], 294))
		})
	})

	t.Run("Unknown command", func(t *testing.T) {
		assert := assert.New(t)

		_, _, err := commandPacketType(packet{
			header{
				version:   1,
				isCommand: true,
				number:    420,
			},
			[]byte{0},
		})

		assert.ErrorIs(err, errUnknownCommand)
	})
}

func TestDataPacket(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		assert := assert.New(t)

		ps, nextPacket := dataIntoPackets(52, []byte("Hello from server!"))

		assert.EqualValues(53, nextPacket)
		assert.Len(ps, 1)
		p := ps[0]
		assert.EqualValues(52, p.number)
		assert.False(p.isCommand)
		assert.Equal([]byte("Hello from server!"), p.data)
	})

	t.Run("Big packet", func(t *testing.T) {
		assert := assert.New(t)

		ps, nextPacket := dataIntoPackets(98, []byte(strings.Repeat("f", maxDataSize)))

		assert.EqualValues(99, nextPacket)
		assert.Len(ps, 1)
		p := ps[0]
		assert.EqualValues(98, p.number)
		assert.False(p.isCommand)
		assert.Equal([]byte(strings.Repeat("f", maxDataSize)), p.data)
	})

	t.Run("Muiltiple packets", func(t *testing.T) {
		assert := assert.New(t)

		ps, nextPacket := dataIntoPackets(37, []byte(strings.Repeat("a", maxDataSize)+
			strings.Repeat("b", maxDataSize)+strings.Repeat("c", maxDataSize)+
			strings.Repeat("d", 228)))

		assert.EqualValues(41, nextPacket)
		p := ps[0]
		assert.EqualValues(37, p.number)
		assert.False(p.isCommand)
		assert.Equal([]byte(strings.Repeat("a", maxDataSize)), p.data)
		p = ps[1]
		assert.EqualValues(38, p.number)
		assert.False(p.isCommand)
		assert.Equal([]byte(strings.Repeat("b", maxDataSize)), p.data)
		p = ps[2]
		assert.EqualValues(39, p.number)
		assert.False(p.isCommand)
		assert.Equal([]byte(strings.Repeat("c", maxDataSize)), p.data)
		p = ps[3]
		assert.EqualValues(40, p.number)
		assert.False(p.isCommand)
		assert.Equal([]byte(strings.Repeat("d", 228)), p.data)
	})

	t.Run("Full flow", func(t *testing.T) {
		assert := assert.New(t)

		for _, data := range [][]byte{
			[]byte("Hello from server!"),
			[]byte(strings.Repeat("a", maxDataSize) + "b"),
			[]byte(strings.Repeat("a", maxDataSize) + strings.Repeat("b", maxDataSize/2)),
			[]byte(strings.Repeat("x", maxDataSize) + strings.Repeat("y", maxDataSize) + "Hello from client!"),
		} {
			ps, _ := dataIntoPackets(284, data)
			var msgs [][]byte
			for _, p := range ps {
				msg := make([]byte, maxPacketSize)
				n, err := p.encode(msg)
				assert.NoError(err)
				msgs = append(msgs, msg[:n])
			}

			var receive []byte
			for i, msg := range msgs {
				p, err := decodePacket(msg)
				assert.NoError(err)
				assert.EqualValues(284+i, p.number)
				assert.False(p.isCommand)
				receive = append(receive, p.data...)
			}

			assert.Equal(data, receive)
		}
	})
}

func TestPacket_Len(t *testing.T) {
	t.Run("Packet should encode its len", func(t *testing.T) {
		assert := assert.New(t)

		p := dataPacket(420, bytes.Repeat([]byte{1}, 1))
		buf := make([]byte, maxPacketSize)
		n, err := p.encode(buf)
		assert.Equal(p.len(), n)
		assert.NoError(err)

		p = dataPacket(420, bytes.Repeat([]byte{2}, 678))
		buf = make([]byte, maxPacketSize)
		n, err = p.encode(buf)
		assert.Equal(p.len(), n)
		assert.NoError(err)

		p = dataPacket(420, bytes.Repeat([]byte{3}, maxDataSize))
		buf = make([]byte, maxPacketSize)
		n, err = p.encode(buf)
		assert.Equal(p.len(), n)
		assert.NoError(err)
	})
	t.Run("Packet len should be exactly enough for encoding", func(t *testing.T) {
		assert := assert.New(t)

		p := dataPacket(69, bytes.Repeat([]byte{1}, 1))
		buf := make([]byte, p.len())
		_, err := p.encode(buf)
		assert.NoError(err)
		assert.EqualValues(1, buf[len(buf)-1])

		p = dataPacket(69, bytes.Repeat([]byte{2}, 678))
		buf = make([]byte, p.len())
		_, err = p.encode(buf)
		assert.NoError(err)
		assert.EqualValues(2, buf[len(buf)-1])

		p = dataPacket(69, bytes.Repeat([]byte{3}, maxDataSize))
		buf = make([]byte, p.len())
		_, err = p.encode(buf)
		assert.NoError(err)
		assert.EqualValues(3, buf[len(buf)-1])
	})
}

func TestUint20_Overflow(t *testing.T) {
	maxUint20 := uint32(1<<20 - 1)

	assert := assert.New(t)

	assert.Panics(func() {
		_ = closeConnectionPacket(maxUint20 + 1)
	})

	assert.Panics(func() {
		_ = receivedPacketsPacket(maxUint20+1, nil)
	})
	assert.Panics(func() {
		_ = receivedPacketsPacket(1, []rng[uint32]{{maxUint20 + 1, 1}})
	})
	assert.Panics(func() {
		_ = receivedPacketsPacket(1, []rng[uint32]{{1, maxUint20 + 1}})
	})
	assert.Panics(func() {
		_ = receivedPacketsPacket(1, []rng[uint32]{{1, 2}, {2, maxUint20 + 1}, {4, 5}})
	})
	assert.Panics(func() {
		_ = receivedPacketsPacket(1, []rng[uint32]{{1, 2}, {2, 4}, {5, maxUint20 + 1}})
	})

	assert.Panics(func() {
		_, _ = dataIntoPackets(maxUint20+1, []byte{1, 2, 3})
	})
}

func testDecodeReceivedPackets(t *testing.T, p packet) []rng[uint32] {
	t.Helper()
	assert := assert.New(t)

	assert.True(p.isCommand)
	tp, payload, err := commandPacketType(p)
	assert.NoError(err)
	assert.Equal(commandReceivedPackets, tp)
	recieved, err := decodeReceivedPackets(payload)
	assert.NoError(err)
	return recieved
}
