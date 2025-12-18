package sudp

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGroup_Packets(t *testing.T) {
	t.Run("Should send right data packets", func(t *testing.T) {
		assert := assert.New(t)
		ps := &testPacketBuffer{
			t: t,
		}
		sended := &[]rng[uint32]{{0, 100}}
		g := newGroup(ps, func() {}, make(chan struct{}), &sync.RWMutex{}, sended, 420)

		ok, n, err := g.appendAndSend([]byte(strings.Repeat("A", maxDataSize) +
			strings.Repeat("B", maxDataSize) + strings.Repeat("C", maxDataSize/2)))
		assert.True(ok)
		assert.Equal(maxDataSize*2+maxDataSize/2, n)
		assert.NoError(err)
		ok, n, err = g.appendAndSend([]byte(strings.Repeat("D", maxDataSize/5)))
		assert.True(ok)
		assert.Equal(maxDataSize/5, n)
		assert.NoError(err)

		packets := ps.Packets()
		assert.Len(packets, 4)
		assert.EqualValues(420, packets[0].number)
		assert.False(packets[0].isCommand)
		assert.Equal([]byte(strings.Repeat("A", maxDataSize)), packets[0].data)
		assert.EqualValues(421, packets[1].number)
		assert.False(packets[0].isCommand)
		assert.Equal([]byte(strings.Repeat("B", maxDataSize)), packets[1].data)
		assert.EqualValues(422, packets[2].number)
		assert.False(packets[0].isCommand)
		assert.Equal([]byte(strings.Repeat("C", maxDataSize/2)), packets[2].data)
		assert.EqualValues(423, packets[3].number)
		assert.False(packets[0].isCommand)
		assert.Equal([]byte(strings.Repeat("D", maxDataSize/5)), packets[3].data)
	})
}

func TestGroup_Timers(t *testing.T) {
	t.Run("Should resend packets after short timer if something is not received and no new data in small window", func(t *testing.T) {
		assert := assert.New(t)
		ps := &testPacketBuffer{
			t: t,
		}
		sendedMu := &sync.RWMutex{}
		sended := &[]rng[uint32]{{33, 34}, {37, 37}}
		g := newGroup(ps, func() {}, make(chan struct{}), sendedMu, sended, 33)

		ok, _, err := g.appendAndSend([]byte("Hello"))
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte(", World"))
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte("!\n"))
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte("What is "))
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte("your name?\n"))
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte("My name is 5aradise!\n"))
		assert.True(ok)
		assert.NoError(err)

		time.Sleep(sShortTime)

		time.Sleep(deliveryDelay / 2)

		ok, _, _ = g.appendAndSend([]byte("smth"))
		assert.False(ok, "should be closed for new messages after short timer")

		sendedMu.Lock()
		*sended = []rng[uint32]{{33, 38}}
		sendedMu.Unlock()

		packets := ps.Packets()
		// sended packets shuld be [33, 34, 35, 36, 37, 38, (35, 36, 38) - it was not in sended]
		assert.GreaterOrEqual(len(packets), 9)
		assert.EqualValues(33, packets[0].number)
		assert.EqualValues(34, packets[1].number)
		assert.EqualValues(35, packets[2].number)
		assert.EqualValues(36, packets[3].number)
		assert.EqualValues(37, packets[4].number)
		assert.EqualValues(38, packets[5].number)
		var res []byte
		for _, p := range packets[:6] {
			res = append(res, p.data...)
		}
		assert.Equal("Hello, World!\nWhat is your name?\nMy name is 5aradise!\n", string(res))

		assert.EqualValues(35, packets[6].number)
		assert.EqualValues(36, packets[7].number)
		assert.EqualValues(38, packets[8].number)
	})

	t.Run("Should resend packets after long timer if something is not received and new data in small window", func(t *testing.T) {
		assert := assert.New(t)
		ps := &testPacketBuffer{
			t: t,
		}
		sendedMu := &sync.RWMutex{}
		sended := &[]rng[uint32]{{33, 34}, {37, 37}, {39, 100}}
		smallWindow := sShortTime / 2
		smallWindowPackets := int(sLongTime / smallWindow)
		restToTime := sLongTime - smallWindow*time.Duration(smallWindowPackets)
		g := newGroup(ps, func() {}, make(chan struct{}), sendedMu, sended, 33)

		for i := range smallWindowPackets {
			ok, _, err := g.appendAndSend([]byte{byte(i)})
			assert.True(ok)
			assert.NoError(err)

			time.Sleep(smallWindow)
		}
		time.Sleep(restToTime)

		time.Sleep(deliveryDelay / 2)

		ok, _, _ := g.appendAndSend([]byte("smth"))
		assert.False(ok, "should be closed for new messages after long timer")

		sendedMu.Lock()
		*sended = []rng[uint32]{{33, 100}}
		sendedMu.Unlock()

		packets := ps.Packets()
		// sended packets shuld be [33, 34, 35, 36, 37, 38, 39, 40, (35, 36, 38) - it was not in sended]
		assert.GreaterOrEqual(len(packets), smallWindowPackets+3)
		for i := range smallWindowPackets {
			assert.EqualValues(33+i, packets[i].number)
		}

		assert.EqualValues(35, packets[smallWindowPackets].number)
		assert.EqualValues(36, packets[smallWindowPackets+1].number)
		assert.EqualValues(38, packets[smallWindowPackets+2].number)
	})
}

func TestGroup_Resending(t *testing.T) {
	t.Run("If something is not received should at least 3 times resend", func(t *testing.T) {
		assert := assert.New(t)
		ps := &testPacketBuffer{
			t: t,
		}
		sendedMu := &sync.RWMutex{}
		sended := &[]rng[uint32]{{34, 35}}
		g := newGroup(ps, func() {}, make(chan struct{}), sendedMu, sended, 33)
		ok, _, err := g.appendAndSend([]byte{0})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{1})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{2})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{3})
		assert.True(ok)
		assert.NoError(err)

		time.Sleep(sShortTime * 10)

		time.Sleep(deliveryDelay / 2)

		packets := ps.Packets()
		// sended packets shuld be [33, 34, 35, 36, (33, 36, 33, 36, 33, 36) - it was not in sended]
		assert.Len(packets, 10)
		assert.EqualValues(33, packets[0].number)
		assert.EqualValues(34, packets[1].number)
		assert.EqualValues(35, packets[2].number)
		assert.EqualValues(36, packets[3].number)

		assert.EqualValues(33, packets[4].number)
		assert.EqualValues(36, packets[5].number)

		assert.EqualValues(33, packets[6].number)
		assert.EqualValues(36, packets[7].number)

		assert.EqualValues(33, packets[8].number)
		assert.EqualValues(36, packets[9].number)
	})

	t.Run("If something is not received after 3 resends should close connection", func(t *testing.T) {
		assert := assert.New(t)
		ps := &testPacketBuffer{
			t: t,
		}
		sendedMu := &sync.RWMutex{}
		sended := &[]rng[uint32]{{34, 35}}
		var closedConn atomic.Bool
		closeConn := func() { closedConn.Store(true) }
		g := newGroup(ps, closeConn, make(chan struct{}), sendedMu, sended, 33)

		ok, _, err := g.appendAndSend([]byte{0})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{1})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{2})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{3})
		assert.True(ok)
		assert.NoError(err)

		time.Sleep(sShortTime * 10)

		time.Sleep(sShortTime * 10)

		time.Sleep(deliveryDelay / 2)

		assert.True(closedConn.Load())
	})

	t.Run("If all packets are received after 3 resends should not close connection", func(t *testing.T) {
		assert := assert.New(t)
		ps := &testPacketBuffer{
			t: t,
		}
		sendedMu := &sync.RWMutex{}
		sended := &[]rng[uint32]{{34, 35}}
		var closedConn atomic.Bool
		closeConn := func() { closedConn.Store(true) }
		g := newGroup(ps, closeConn, make(chan struct{}), sendedMu, sended, 33)

		ok, _, err := g.appendAndSend([]byte{0})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{1})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{2})
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{3})
		assert.True(ok)
		assert.NoError(err)

		time.Sleep(sShortTime * 10)

		sendedMu.Lock()
		*sended = []rng[uint32]{{33, 46}}
		sendedMu.Unlock()

		time.Sleep(sShortTime * 10)

		time.Sleep(deliveryDelay / 2)

		assert.Len(ps.Packets(), 4+2+2+2)
		assert.False(closedConn.Load())
	})
}

func TestGroup_IncNextPacket(t *testing.T) {
	t.Run("Incrementin dont affect on packet resending", func(t *testing.T) {
		assert := assert.New(t)
		ps := &testPacketBuffer{
			t: t,
		}
		sendedMu := &sync.RWMutex{}
		sended := &[]rng[uint32]{{33, 34}, {36, 37}}
		var closedConn atomic.Bool
		closeConn := func() { closedConn.Store(true) }
		g := newGroup(ps, closeConn, make(chan struct{}), sendedMu, sended, 33)

		ok, _, err := g.appendAndSend([]byte{0}) // 33
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{1}) // 34
		assert.True(ok)
		assert.NoError(err)
		nextPacket := g.incNextPacket() // 35 (dont care)
		assert.EqualValues(35, nextPacket)
		ok, _, err = g.appendAndSend([]byte{2}) // 36
		assert.True(ok)
		assert.NoError(err)
		ok, _, err = g.appendAndSend([]byte{3}) // 37
		assert.True(ok)
		assert.NoError(err)
		nextPacket = g.incNextPacket() // 38 (dont care)
		assert.EqualValues(38, nextPacket)

		time.Sleep(sShortTime)

		time.Sleep(deliveryDelay / 2)

		assert.Len(ps.Packets(), 4)
	})
}

type testPacketBuffer struct {
	t       *testing.T
	mu      sync.Mutex
	packets []packet
}

func (buf *testPacketBuffer) Write(b []byte) (int, error) {
	assert := assert.New(buf.t)

	p, err := decodePacket(b)
	assert.NoError(err)
	buf.mu.Lock()
	buf.packets = append(buf.packets, p)
	buf.mu.Unlock()
	return len(b), nil
}

func (buf *testPacketBuffer) Packets() []packet {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	return slices.Clone(buf.packets)
}
