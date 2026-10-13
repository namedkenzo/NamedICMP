// Package wire implements the documented PingTunnel wire format independently.
// It deliberately has no dependency on PingTunnel or gohome source code.
package wire

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"
)

var ErrMalformed = errors.New("malformed tunnel packet")

const MaxMessage = 16384
const Magic = 0xdead
const Ring = 1000000
const PayloadSize = 888

type field struct {
	n, kind int
	v       uint64
	b       []byte
}

func parse(b []byte) ([]field, error) {
	if len(b) > MaxMessage {
		return nil, ErrMalformed
	}
	var out []field
	for len(b) > 0 {
		tag, n := binary.Uvarint(b)
		if n <= 0 || tag>>3 == 0 || tag>>3 > 536870911 || len(out) >= 128 {
			return nil, ErrMalformed
		}
		b = b[n:]
		f := field{n: int(tag >> 3), kind: int(tag & 7)}
		switch f.kind {
		case 0:
			v, k := binary.Uvarint(b)
			if k <= 0 {
				return nil, ErrMalformed
			}
			f.v = v
			b = b[k:]
		case 1:
			if len(b) < 8 {
				return nil, ErrMalformed
			}
			b = b[8:]
		case 2:
			v, k := binary.Uvarint(b)
			if k <= 0 || v > uint64(len(b)-k) {
				return nil, ErrMalformed
			}
			b = b[k:]
			f.b = b[:int(v)]
			b = b[int(v):]
		case 5:
			if len(b) < 4 {
				return nil, ErrMalformed
			}
			b = b[4:]
		default:
			return nil, ErrMalformed
		}
		out = append(out, f)
	}
	return out, nil
}
func integer(b []byte, n int, v uint64) []byte {
	if v == 0 {
		return b
	}
	b = binary.AppendUvarint(b, uint64(n<<3))
	return binary.AppendUvarint(b, v)
}
func bytesField(b []byte, n int, v []byte) []byte {
	if len(v) == 0 {
		return b
	}
	b = binary.AppendUvarint(b, uint64(n<<3|2))
	b = binary.AppendUvarint(b, uint64(len(v)))
	return append(b, v...)
}
func zig(v int32) uint64   { return uint64(uint32(v<<1) ^ uint32(v>>31)) }
func unzig(v uint64) int32 { return int32(uint32(v>>1) ^ uint32(-int32(v&1))) }

type Message struct {
	Connection  string
	Kind        int32
	Target      string
	Payload     []byte
	ReplyType   int32
	Key         int32
	Timeout     int32
	TCP         bool
	Buffer      int32
	Window      int32
	ResendMS    int32
	Compression int32
}

func DecodeMessage(b []byte) (Message, error) {
	var m Message
	fs, err := parse(b)
	if err != nil {
		return m, err
	}
	magic := int32(0)
	for _, f := range fs {
		switch f.n {
		case 1, 3, 4:
			if f.kind != 2 {
				return m, ErrMalformed
			}
			switch f.n {
			case 1:
				m.Connection = string(f.b)
			case 3:
				m.Target = string(f.b)
			case 4:
				m.Payload = f.b
			}
		case 2, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14:
			if f.kind != 0 {
				return m, ErrMalformed
			}
			switch f.n {
			case 2:
				m.Kind = int32(f.v)
			case 5:
				m.ReplyType = unzig(f.v)
			case 6:
				magic = unzig(f.v)
			case 7:
				m.Key = unzig(f.v)
			case 8:
				m.Timeout = int32(f.v)
			case 9:
				m.TCP = f.v > 0
			case 10:
				m.Buffer = int32(f.v)
			case 11:
				m.Window = int32(f.v)
			case 12:
				m.ResendMS = int32(f.v)
			case 13:
				m.Compression = int32(f.v)
			}
		}
	}
	if magic != Magic || m.Kind < 0 || m.Kind > 2 || len(m.Connection) > 160 || len(m.Target) > 300 || !utf8.ValidString(m.Connection) || !utf8.ValidString(m.Target) {
		return m, ErrMalformed
	}
	return m, nil
}
func EncodeMessage(m Message) []byte {
	b := make([]byte, 0, len(m.Payload)+100)
	b = bytesField(b, 1, []byte(m.Connection))
	b = integer(b, 2, uint64(m.Kind))
	b = bytesField(b, 3, []byte(m.Target))
	b = bytesField(b, 4, m.Payload)
	b = integer(b, 5, zig(m.ReplyType))
	b = integer(b, 6, zig(Magic))
	b = integer(b, 7, zig(m.Key))
	b = integer(b, 8, uint64(m.Timeout))
	if m.TCP {
		b = integer(b, 9, 1)
	}
	b = integer(b, 10, uint64(m.Buffer))
	b = integer(b, 11, uint64(m.Window))
	b = integer(b, 12, uint64(m.ResendMS))
	b = integer(b, 13, uint64(m.Compression))
	return b
}

type Frame struct {
	Kind       int32
	ID         int32
	Time       int64
	Resend     bool
	DataKind   int32
	Data       []byte
	Compressed bool
	IDs        []int32
}

func DecodeFrame(b []byte) (Frame, error) {
	var f Frame
	fs, err := parse(b)
	if err != nil {
		return f, err
	}
	hasData := false
	for _, v := range fs {
		switch v.n {
		case 1, 2, 3, 4, 7:
			if v.kind != 0 {
				return f, ErrMalformed
			}
			switch v.n {
			case 1:
				f.Kind = int32(v.v)
			case 2:
				f.Resend = v.v > 0
			case 3:
				f.Time = int64(v.v)
			case 4:
				f.ID = int32(v.v)
			}
		case 5:
			if v.kind != 2 {
				return f, ErrMalformed
			}
			hasData = true
			ds, e := parse(v.b)
			if e != nil {
				return f, e
			}
			for _, d := range ds {
				switch d.n {
				case 1:
					if d.kind != 0 {
						return f, ErrMalformed
					}
					f.DataKind = int32(d.v)
				case 2:
					if d.kind != 2 {
						return f, ErrMalformed
					}
					f.Data = d.b
				case 3:
					if d.kind != 0 {
						return f, ErrMalformed
					}
					f.Compressed = d.v != 0
				}
			}
		case 6:
			if v.kind == 0 {
				f.IDs = append(f.IDs, int32(v.v))
			} else if v.kind == 2 {
				for p := v.b; len(p) > 0; {
					n, k := binary.Uvarint(p)
					if k <= 0 || len(f.IDs) >= 256 {
						return f, ErrMalformed
					}
					f.IDs = append(f.IDs, int32(n))
					p = p[k:]
				}
			} else {
				return f, ErrMalformed
			}
		}
	}
	if f.Kind < 0 || f.Kind > 4 || f.ID < 0 || f.ID >= Ring || f.DataKind < 0 || f.DataKind > 4 || f.Kind == 0 && !hasData || len(f.IDs) > 256 {
		return f, ErrMalformed
	}
	for _, id := range f.IDs {
		if id < 0 || id >= Ring {
			return f, ErrMalformed
		}
	}
	return f, nil
}
func EncodeFrame(f Frame) []byte {
	var b []byte
	b = integer(b, 1, uint64(f.Kind))
	if f.Resend {
		b = integer(b, 2, 1)
	}
	b = integer(b, 3, uint64(f.Time))
	b = integer(b, 4, uint64(f.ID))
	if f.Kind == 0 {
		var d []byte
		d = integer(d, 1, uint64(f.DataKind))
		d = bytesField(d, 2, f.Data)
		if f.Compressed {
			d = integer(d, 3, 1)
		}
		b = binary.AppendUvarint(b, 42)
		b = binary.AppendUvarint(b, uint64(len(d)))
		b = append(b, d...)
	}
	if len(f.IDs) > 0 {
		var p []byte
		for _, id := range f.IDs {
			p = binary.AppendUvarint(p, uint64(id))
		}
		b = bytesField(b, 6, p)
	}
	return b
}
