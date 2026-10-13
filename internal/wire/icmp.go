package wire

import "encoding/binary"

func Checksum(b []byte) uint16 {
	var sum uint32
	for len(b) > 1 {
		sum += uint32(binary.BigEndian.Uint16(b))
		b = b[2:]
	}
	if len(b) > 0 {
		sum += uint32(b[0]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 65535) + (sum >> 16)
	}
	return ^uint16(sum)
}
func Echo(typ byte, id, seq uint16, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	b[0] = typ
	binary.BigEndian.PutUint16(b[4:6], id)
	binary.BigEndian.PutUint16(b[6:8], seq)
	copy(b[8:], payload)
	binary.BigEndian.PutUint16(b[2:4], Checksum(b))
	return b
}
func DecodeEcho(b []byte) (byte, uint16, uint16, []byte, error) {
	// Linux IPConn strips the IPv4 header; test/replay inputs may include one.
	if len(b) >= 20 && b[0]>>4 == 4 {
		n := int(b[0]&15) * 4
		if n < 20 || n > len(b) || b[9] != 1 || binary.BigEndian.Uint16(b[6:8])&0x3fff != 0 {
			return 0, 0, 0, nil, ErrMalformed
		}
		b = b[n:]
	}
	if len(b) < 8 || b[1] != 0 || (b[0] != 0 && b[0] != 8) || Checksum(b) != 0 {
		return 0, 0, 0, nil, ErrMalformed
	}
	return b[0], binary.BigEndian.Uint16(b[4:6]), binary.BigEndian.Uint16(b[6:8]), b[8:], nil
}
