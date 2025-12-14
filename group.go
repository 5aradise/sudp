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
	packets    [][]byte // mark sent messages by setting them to nil
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
func (g *group) appendAndSendReceived(receivedPackets []rng[uint32]) (ok bool, err error) {
	if !g.short.Reset(sShortTime) {
		return false, nil
	}

	g.packetsMu.Lock()
	defer g.packetsMu.Unlock()

	packetNum := g.nextPacket

	p := receivedPacketsPacket(packetNum, receivedPackets)
	data := make([]byte, p.len())
	packetSize, err := p.encode(data)
	if err != nil { // should never happen
		panic(err)
	}
	data = data[:packetSize]

	written, err := g.w.Write(data)
	if err != nil {
		return true, fmt.Errorf("failed to write to main connection: %w", err)
	}
	if written != packetSize {
		return true, ErrPacketCorrupted
	}
	g.packets = append(g.packets, data)

	g.nextPacket = packetNum + 1
	return true, nil
}

// ok indicates whether the group can accept new packets
func (g *group) appendAndSendData(data []byte) (ok bool, n int, err error) {
	if !g.short.Reset(sShortTime) {
		return false, 0, nil
	}

	g.packetsMu.Lock()
	defer g.packetsMu.Unlock()

	ps, nextPacket := dataIntoPackets(g.nextPacket, data)
	g.packets = slices.Grow(g.packets, len(ps))
	for _, p := range ps {
		data := make([]byte, p.len())
		packetSize, err := p.encode(data)
		if err != nil { // should never happen
			panic(err)
		}
		data = data[:packetSize]

		written, err := g.w.Write(data)
		if err != nil {
			return true, 0, fmt.Errorf("failed to write to main connection: %w", err)
		}
		if written != packetSize {
			return true, 0, ErrPacketCorrupted
		}
		n += len(p.data)
		g.packets = append(g.packets, data)
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
	var hasUnconfirmed bool
	resendDelay := sShortTime
	g.packetsMu.Lock()
	for range resendTries {
		select {
		case <-g.stopCh:
			return
		default:
		}

		g.markSended()
		for _, p := range g.packets {
			if p == nil {
				continue
			}
			hasUnconfirmed = true
			_, err := g.w.Write(p)
			if err != nil {
				return
			}
		}

		if !hasUnconfirmed {
			return
		}

		resendDelay *= 2
		time.Sleep(resendDelay)
	}

	g.markSended()
	for _, p := range g.packets {
		if p != nil {
			g.closeConn()
			return
		}
	}
}

func (g *group) markSended() {
	g.sendedMu.RLock()
	for _, r := range *g.sended {
		s := len(g.packets) - (int(g.nextPacket) - int(r[0]))
		if s >= len(g.packets) {
			continue
		}
		e := len(g.packets) - (int(g.nextPacket) - int(r[1]))
		if e < 0 {
			continue
		}
		clear(g.packets[max(s, 0):min(e+1, len(g.packets))])
	}
	g.sendedMu.RUnlock()
}
