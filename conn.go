package sudp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

/*
Since packets will be dropped when the buffer is full,
it is important to calculate its optimal size.

packet flow: ... -> conn  -> incompleteOrder -(if data)-> bufQueue -> user
time:		 ... -> short -> instant                   -> long

avg time for command: short
avg time for data:    short + long

buffering for cmd: 	connCap
buffering for data: connCap + userCap

connCap - shuld be small
userCap - shuld be big
*/
const (
	// number of packets that conn may not read before blocking
	// (time for package to process)
	connCap = 256
	// number of packets that client may not read before blocking
	// (time for client to process)
	userCap = 4096

	rShortTime = 300 * time.Millisecond
	rLongTime  = 3 * time.Second
)

var (
	ErrPacketCorrupted = errors.New("packet corrupted while writing")
	errCloseFuncCalled = fmt.Errorf("%w: close function called", net.ErrClosed)
	errRemotelyClosed  = fmt.Errorf("%w: remotely closed", net.ErrClosed)
	errNoResponse      = fmt.Errorf("%w: no response", net.ErrClosed)
)

// Errors:
// An error in reading and writing may occur for two reasons related to the connection:
// 1. It is closed (both directions must be notified at once)
// 2. Problems with the external main connection
// (the error will appear when attempting to interact with the external connection,
// so there is no need to notify the other direction)

// Packet number tracking:
// The problem is that our protocol does not need to track the numbers of received and
// sent packets at least for now, because they do not need to be resent and checked for delivery.
// Moreover, when they are present, if the number of a packet is missing,
// we cannot read the next received packets, because we cannot be sure that the packet
// that was not received is a letter of received packets.
type conn struct {
	out struct {
		r    <-chan []byte
		rerr *error // will be set after close r channel

		w io.Writer

		close func() error
	}

	internalErr atomic.Bool
	closeErr    atomic.Value // should be specified before closing other components

	// write
	stopGroups    chan struct{}
	sendedMu      *sync.RWMutex
	sendedVersion uint32 // in case we receive a packet with an old version becous of missorder
	sended        *[]rng[uint32]
	// comunication with other groups should be through
	// stop channel and pointer to sended packets
	lastGroupMu sync.Mutex
	lastGroup   *group

	// read
	toRead     *bufQueue
	short      *time.Timer
	long       *time.Timer
	receivedMu sync.RWMutex
	nextRecivP uint32
	received   []rng[uint32]
	unreaded   incompleteOrder
}

// - if the problem is with the external connection,
// then inerr should be specified the connection error value, and the channel should be closed
// - if the connection is closed for an internal reason,
// then nil should be sent through the channel, and rerr is not expected to be specified
func newConn(in <-chan []byte, inerr *error, out io.Writer, onClose func() error) *conn {
	if onClose == nil {
		onClose = func() error { return nil }
	}
	c := &conn{
		toRead: newBufQueue(userCap),
		out: struct {
			r     <-chan []byte
			rerr  *error
			w     io.Writer
			close func() error
		}{in, inerr, out, onClose},
		stopGroups: make(chan struct{}),
		sendedMu:   &sync.RWMutex{},
		sended:     new([]rng[uint32]),
	}
	c.short = time.AfterFunc(time.Hour, c.shortTFunc)
	c.short.Stop()
	c.long = time.AfterFunc(time.Hour, c.longTFunc)
	c.long.Stop()
	go func() {
		err := c.run()
		if err != nil {
			if !c.internalErr.Load() {
				c.toRead.close(err)
			}

			c.close(fmt.Errorf("running the connection: %w", err), false)
		} else {
			c.toRead.close(c.closeErr.Load().(error))
		}
	}()
	return c
}

func (c *conn) Read(b []byte) (int, error) {
	if clErr := c.closeErr.Load(); clErr != nil {
		return 0, clErr.(error)
	}

	return c.toRead.read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	if clErr := c.closeErr.Load(); clErr != nil {
		return 0, clErr.(error)
	}

	ok, n, err := c.group().appendAndSend(b)
	if !ok {
		_, n, err = c.nextGroup().appendAndSend(b)
	}
	if err != nil {
		c.close(fmt.Errorf("writing: %w", err), false)
	}
	return n, err
}

func (c *conn) Close() error {
	return c.close(errCloseFuncCalled, true)
}

// reading

func (c *conn) run() error {
	for {
		data, internalErr := <-c.out.r
		if !internalErr { // external connection error
			return fmt.Errorf("failed to read from main connection: %w", *c.out.rerr)
		}
		if data == nil { // internal connection closed
			return nil
		}

		p, err := decodePacket(data)
		if err != nil {
			return fmt.Errorf("invalid packet: %w", err)
		}
		var (
			command command = -1
			payload []byte
		)
		if p.isCommand {
			command, payload, err = commandPacketType(p)
			if err != nil {
				return err
			}
		}
		cmdReceivedPackets := command == commandReceivedPackets

		if !cmdReceivedPackets {
			if !c.addToReceived(p.number) {
				err := c.sendReceivedPackets()
				if err != nil {
					return fmt.Errorf("failed to send received packets: %w", err)
				}
				continue
			}
		}

		if p.isCommand {
			err := c.handleCommand(p.number, command, payload)
			if err != nil {
				return fmt.Errorf("failed to handle command: %w", err)
			}
		}

		if !cmdReceivedPackets {
			for toRead := range c.unreaded.append(p) {
				c.toRead.write(toRead)
			}
		}
	}
}

func (c *conn) handleCommand(number uint32, command command, payload []byte) error {
	switch command {
	case commandCloseConn:
		outErr := c.closeLocaly(errRemotelyClosed, false)
		return outErr
	case commandReceivedPackets:
		sended, err := decodeReceivedPackets(payload)
		if err != nil {
			return err
		}
		c.markSendedPackets(number, sended)
		return nil
	default:
		panic("unknown command") // should never happen
	}
}

func (c *conn) markSendedPackets(version uint32, sended []rng[uint32]) {
	c.sendedMu.Lock()
	defer c.sendedMu.Unlock()

	if version > c.sendedVersion {
		c.sendedVersion = version
		*c.sended = sended
	}
}

func (c *conn) addToReceived(number uint32) (added bool) {
	c.receivedMu.Lock()
	c.received, added = rangesTryAppend(c.received, number)
	c.receivedMu.Unlock()

	if added {
		if !c.short.Reset(rShortTime) { // one of previous timers was excided, so start new cicle
			c.long.Reset(rLongTime)
		}
	}
	return added
}

func (c *conn) shortTFunc() {
	if !c.long.Stop() {
		return
	}

	c.sendReceivedPackets()
}

func (c *conn) longTFunc() {
	if !c.short.Stop() {
		return
	}

	c.sendReceivedPackets()
}

// close

func (c *conn) close(why error, internal bool) error {
	var sendCloseErr error
	if c.closeErr.Load() == nil {
		sendCloseErr = c.sendCloseCommand()
	}
	closeLocalErr := c.closeLocaly(why, internal)
	return errors.Join(closeLocalErr, sendCloseErr)
}

func (c *conn) closeLocaly(why error, internal bool) (outErr error) {
	prevErr := c.closeErr.Load()
	if internal {
		c.internalErr.Store(true)
		c.closeErr.Store(why)
	} else if !c.internalErr.Load() {
		c.closeErr.Store(why)
	}
	if prevErr == nil { // first call
		close(c.stopGroups)
		return c.out.close()
	}
	return nil
}

func (c *conn) closeOnNoResponse() {
	c.close(errNoResponse, false)
}

// commands

func (c *conn) sendReceivedPackets() error {
	c.receivedMu.Lock()
	p := receivedPacketsPacket(c.nextRecivP, c.received)
	c.nextRecivP++
	c.receivedMu.Unlock()
	return c.sendPacketOutOfGroup(p)
}

// dont add to any group, because command should be sent only once
func (c *conn) sendCloseCommand() error {
	packetNum := c.nextPacketNum()
	p := closeConnectionPacket(packetNum)
	return c.sendPacketOutOfGroup(p)
}

func (c *conn) sendPacketOutOfGroup(p packet) error {
	if clErr := c.closeErr.Load(); clErr != nil {
		return clErr.(error)
	}

	data := make([]byte, p.len())
	packetSize, err := p.encode(data)
	if err != nil { // should never happen
		panic(err)
	}
	data = data[:packetSize]

	written, err := c.out.w.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to main connection: %w", err)
	}
	if written != packetSize {
		return ErrPacketCorrupted
	}
	return nil
}

func (c *conn) nextPacketNum() uint32 {
	c.lastGroupMu.Lock()
	defer c.lastGroupMu.Unlock()

	if c.lastGroup != nil {
		return c.lastGroup.incNextPacket()
	}
	return 0
}

// groups

func (c *conn) group() *group {
	c.lastGroupMu.Lock()
	defer c.lastGroupMu.Unlock()

	if c.lastGroup == nil {
		g := newGroup(c.out.w, c.closeOnNoResponse,
			c.stopGroups, c.sendedMu, c.sended, 0)
		c.lastGroup = g
		return g
	}

	return c.lastGroup
}

func (c *conn) nextGroup() *group {
	c.lastGroupMu.Lock()
	defer c.lastGroupMu.Unlock()

	g := newGroup(c.out.w, c.closeOnNoResponse,
		c.stopGroups, c.sendedMu, c.sended, c.lastGroup.nextPacket)
	c.lastGroup = g
	return g
}
