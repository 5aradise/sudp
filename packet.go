package sudp

import (
	"errors"
)

/*
	The packet package tries not to allocate memory where possible,
	leaving the task of memory management to the packet manager

	Version 1 packet format:
    0               1               2
    0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |  ver  |*|               number                |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                     data                      |
   +                     ....                      +

   * - is_command
*/

type packet struct {
	header        // 3 bytes
	data   []byte // max 1469 bytes
}

type header struct {
	version   byte   // 3 bits
	isCommand bool   // 1 bit
	number    uint32 // uint20
}

// 1500 (MTU) - 20 (IP header) - 8 (UDP header) = 1472 bytes
const (
	maxPacketSize = 1472
	headerSize    = 3
	maxDataSize   = maxPacketSize - headerSize

	maxPacketNumber = 1<<20 - 1

	coloseConnFlag      = 0b10101010
	receivedPacketsFlag = 0b11110000
)

// command packets
type command int

const (
	commandCloseConn command = iota
	commandReceivedPackets
)

var (
	errUnknownCommand     = errors.New("unknown command")
	errInvalidRangeFormat = errors.New("invalid range format")
	errTooSmallPacket     = errors.New("too small packet")
	errTooSmallBuffer     = errors.New("too small buffer")
)

func commandPacketType(p packet) (tp command, payload []byte, err error) {
	switch p.data[0] {
	case coloseConnFlag:
		return commandCloseConn, nil, nil
	case receivedPacketsFlag:
		return commandReceivedPackets, p.data[1:], nil
	default:
		return 0, nil, errUnknownCommand
	}
}

func closeConnectionPacket(number uint32) packet {
	if number > maxPacketNumber {
		panic("uint20 overflow")
	}

	return packet{
		header: header{
			version:   1,
			isCommand: true,
			number:    number,
		},
		data: []byte{coloseConnFlag},
	}
}

// received packets must be described by ranges (with inclusive bounds), for example:
//
// if received packets are 0, 1, 2, 3, 5, 7, 8, 11, 12
//
// ranges are 0-3, 5-5, 7-8, 11-12
func receivedPacketsPacket(number uint32, receivedPackets []rng[uint32]) packet {
	if number > maxPacketNumber {
		panic("uint20 overflow")
	}

	return packet{
		header: header{
			version:   1,
			isCommand: true,
			number:    number,
		},
		data: encodeReceivedPackets(receivedPackets),
	}
}
func encodeReceivedPackets(receivedPackets []rng[uint32]) []byte {
	dataSize := 1 + len(receivedPackets)*5
	if dataSize > maxDataSize {
		panic("data size overflow")
	}

	data := make([]byte, 1+len(receivedPackets)*5)
	data[0] = receivedPacketsFlag
	for i, rng := range receivedPackets {
		dataI := i*5 + 1
		n1, n2 := rng[0], rng[1]
		if n1 > maxPacketNumber || n2 > maxPacketNumber {
			panic("uint20 overflow")
		}
		data[dataI] = byte(n1 >> 12)
		data[dataI+1] = byte(n1 >> 4)
		data[dataI+2] = byte(n1<<4) | byte(n2>>16)&0b00001111
		data[dataI+3] = byte(n2 >> 8)
		data[dataI+4] = byte(n2)
	}
	return data
}

func decodeReceivedPackets(payload []byte) ([]rng[uint32], error) {
	if len(payload)%5 != 0 {
		return nil, errInvalidRangeFormat
	}

	ranges := make([]rng[uint32], 0, len(payload)/5*2)
	for i := 0; i < len(payload); i += 5 {
		n1 := uint32(payload[i])<<12 | uint32(payload[i+1])<<4 | uint32(payload[i+2])>>4
		n2 := uint32(payload[i+2]&0b00001111)<<16 | uint32(payload[i+3])<<8 | uint32(payload[i+4])
		ranges = append(ranges, rng[uint32]{n1, n2})
	}
	return ranges, nil
}

// data packets

func dataIntoPackets(initPacketNumber uint32, data []byte) (packets []packet, nextPacket uint32) {
	newPackets := len(data)/maxDataSize + 1
	if initPacketNumber+uint32(newPackets)-1 > maxPacketNumber {
		panic("uint20 overflow")
	}

	ps := make([]packet, 0, newPackets)
	next := initPacketNumber
	for len(data) > maxDataSize {
		p := dataPacket(next, data[:maxDataSize])
		next++
		ps = append(ps, p)
		data = data[maxDataSize:]
	}
	p := dataPacket(next, data)
	next++
	ps = append(ps, p)
	return ps, next
}
func dataPacket(number uint32, data []byte) packet {
	if len(data) > maxDataSize {
		panic("data size overflow")
	}

	return packet{
		header: header{
			version:   1,
			isCommand: false,
			number:    number,
		},
		data: data,
	}
}

// decoding

func decodePacket(src []byte) (packet, error) {
	if len(src) < headerSize {
		return packet{}, errTooSmallPacket
	}

	return packet{
		header: decodeHeader(src),
		data:   src[3:],
	}, nil
}

func decodeHeader(src []byte) header {
	return header{
		version:   src[0] & 0b11100000 >> 5,
		isCommand: src[0]&0b00010000 != 0,
		number:    uint32(src[0]&0b00001111)<<16 | uint32(src[1])<<8 | uint32(src[2]),
	}
}

// encoding

func (p packet) encode(dst []byte) (int, error) {
	if len(dst) < p.len() {
		return 0, errTooSmallBuffer
	}

	p.header.encode(dst)
	n := copy(dst[headerSize:], p.data)
	return headerSize + n, nil
}

func (h header) encode(dst []byte) {
	dst[0] = h.version << 5
	if h.isCommand {
		dst[0] |= 0b00010000
	}
	dst[0] |= byte(h.number>>16) & 0b00001111
	dst[1] = byte(h.number >> 8)
	dst[2] = byte(h.number)
}

func (p packet) len() int {
	return headerSize + len(p.data)
}
