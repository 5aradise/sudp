package sudp

import (
	"fmt"
	"io"
	"slices"
	"sync"
	"time"
)

// Since creating the logic for analyzing connections and dynamically changing message wait times
// would take as much code as the main logic,
// it was decided to take the average time it takes for a message to ping over relatively long distances
// (source: Gemini)
const (
	deliveryDelay = 100 * time.Millisecond
	sShortTime    = rShortTime + deliveryDelay
	sLongTime     = rLongTime + deliveryDelay

	// will see if this is sufficient through further testing
	resendTries = 3
)

// In order not to overload the connection by saving all groups,
// the responsibility for stopping was transferred to the channel,
// and successfully sent packets are transmitted via a link to their list.
type group struct {
	w         io.Writer
	closeConn func()

	short  *time.Timer
	long   *time.Timer
	stopCh <-chan struct{}

	sendedMu   *sync.RWMutex
	sended     *[]rng[uint32]
	packetsMu  sync.Mutex
	packets    []reusable[[]byte] // mark sent messages by setting them to nil
	nextPacket uint32
}

// [newGroup] creates a group of packets linked by timers that control their transmission.
// The main purpose of the group is to ensure that all messages are delivered.
//
// - In w, the group records packets
//
// - closeConn will be called when some packets fail to be sent even after attempts
//
// - To stop the group, you need to close stop channel.
//
// - With sended, the group will periodically take a list of messages that have already been received by the recipient
//
// - nextPacket - the number of the first packet in the group
func newGroup(w io.Writer, closeConn func(), stop <-chan struct{}, sendedMu *sync.RWMutex, sended *[]rng[uint32], nextPacket uint32) *group {
	g := &group{
		w:         w,
		closeConn: closeConn,

		stopCh: stop,

		sendedMu:   sendedMu,
		sended:     sended,
		nextPacket: nextPacket,
	}

	g.short = time.AfterFunc(sShortTime, g.shortTFunc)
	g.long = time.AfterFunc(sLongTime, g.longTFunc)
	return g
}

// ok indicates whether the group can accept new packets
func (g *group) appendAndSend(data []byte) (ok bool, n int, err error) {
	g.packetsMu.Lock()
	defer g.packetsMu.Unlock()

	if !g.short.Reset(sShortTime) {
		g.short.Stop()
		return false, 0, nil
	}

	// no need in copying data because it will be realocated

	ps, nextPacket := dataIntoPackets(g.nextPacket, data)
	g.packets = slices.Grow(g.packets, len(ps))
	for _, p := range ps {
		buf := getPacketBuf()
		packetSize, err := p.encode(buf.data)
		if err != nil { // should never happen
			panic(err)
		}
		buf.data = buf.data[:packetSize]

		written, err := g.w.Write(buf.data)
		if err != nil {
			buf.free()
			return true, 0, fmt.Errorf("failed to write to main connection: %w", err)
		}
		if written != packetSize {
			buf.free()
			return true, 0, ErrPacketCorrupted
		}
		n += len(p.data)
		g.packets = append(g.packets, buf)
	}
	g.nextPacket = nextPacket
	return true, n, nil
}

func (g *group) shortTFunc() {
	if !g.long.Stop() {
		return
	}

	g.resendUnconfirmed()
}

func (g *group) longTFunc() {
	if !g.short.Stop() {
		return
	}

	g.resendUnconfirmed()
}

func (g *group) resendUnconfirmed() {
	defer func() {
		g.packetsMu.Lock()
		clearPackets(g.packets)
		g.packetsMu.Unlock()
	}()

	resendDelay := sShortTime
	for range resendTries {
		select {
		case <-g.stopCh:
			return
		default:
		}

		var hasUnconfirmed bool
		g.packetsMu.Lock()
		g.markSended()
		for _, p := range g.packets {
			if p.data != nil {
				hasUnconfirmed = true
				_, err := g.w.Write(p.data)
				if err != nil {
					g.packetsMu.Unlock()
					return
				}
			}
		}
		g.packetsMu.Unlock()
		if !hasUnconfirmed {
			return
		}

		resendDelay *= 2
		time.Sleep(resendDelay)
	}

	g.packetsMu.Lock()
	g.markSended()
	for _, p := range g.packets {
		if p.data != nil {
			g.packetsMu.Unlock()
			g.closeConn()
			return
		}
	}
	g.packetsMu.Unlock()
}

func (g *group) markSended() {
	var (
		psLen      = len(g.packets)
		nextPacket = int(g.nextPacket)
	)
	g.sendedMu.RLock()
	for _, r := range *g.sended {
		s := psLen - (nextPacket - int(r[0]))
		if s >= psLen {
			continue
		}
		e := psLen - (nextPacket - int(r[1]))
		if e < 0 {
			continue
		}
		clearPackets(g.packets[max(s, 0):min(e+1, psLen)])
	}
	g.sendedMu.RUnlock()
}

func clearPackets(ps []reusable[[]byte]) {
	for i, p := range ps {
		if p.data != nil {
			p.free()
			ps[i] = reusable[[]byte]{}
		}
	}
}

func (g *group) incNextPacket() (nextPacket uint32) {
	g.packetsMu.Lock()
	defer g.packetsMu.Unlock()

	g.packets = append(g.packets, reusable[[]byte]{})
	nextPacket = g.nextPacket
	g.nextPacket++
	return nextPacket
}
