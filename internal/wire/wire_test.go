package wire

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestGoldenEnvelope(t *testing.T) {
	b, _ := hex.DecodeString("0a0163220568656c6c6f281030dafa063880890f")
	m, err := DecodeMessage(b)
	if err != nil || m.Connection != "c" || string(m.Payload) != "hello" || m.ReplyType != 8 || m.Key != 123456 {
		t.Fatalf("golden envelope: %+v %v", m, err)
	}
	if got := EncodeMessage(m); !bytes.Equal(got, b) {
		t.Fatalf("wire mismatch: %x", got)
	}
}
func TestGoldenFrames(t *testing.T) {
	cases := []struct {
		hex      string
		kind     int32
		dataKind int32
		ids      []int32
	}{{"2a020801", 0, 1, nil}, {"2a020802", 0, 2, nil}, {"08023203008101", 2, 0, []int32{0, 129}}}
	for _, c := range cases {
		b, _ := hex.DecodeString(c.hex)
		f, err := DecodeFrame(b)
		if err != nil || f.Kind != c.kind || f.DataKind != c.dataKind || !bytes.Equal(EncodeFrame(f), b) {
			t.Fatalf("%s: %+v %v", c.hex, f, err)
		}
		if len(f.IDs) != len(c.ids) {
			t.Fatal("ACK vector mismatch")
		}
		for i, id := range c.ids {
			if f.IDs[i] != id {
				t.Fatal("ACK ID mismatch")
			}
		}
	}
}
func TestEchoChecksum(t *testing.T) {
	b := Echo(8, 123, 45, []byte("payload"))
	typ, id, seq, p, err := DecodeEcho(b)
	if err != nil || typ != 8 || id != 123 || seq != 45 || string(p) != "payload" {
		t.Fatal("echo roundtrip")
	}
	b[9] ^= 1
	if _, _, _, _, err := DecodeEcho(b); err == nil {
		t.Fatal("corrupt checksum accepted")
	}
}
func TestMalformed(t *testing.T) {
	for _, b := range [][]byte{{0}, {0x0a, 0xff, 0xff}, {0x30, 0xff}, {0x0b}, make([]byte, MaxMessage+1)} {
		if _, err := DecodeMessage(b); err == nil {
			t.Fatalf("accepted malformed %x", b[:min(8, len(b))])
		}
	}
	if _, err := DecodeFrame([]byte{0x08, 0x08}); err == nil {
		t.Fatal("invalid frame kind accepted")
	}
}
func FuzzDecode(f *testing.F) {
	f.Add([]byte{0x2a, 2, 8, 1})
	f.Add([]byte("random"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = DecodeMessage(b)
		_, _ = DecodeFrame(b)
		_, _, _, _, _ = DecodeEcho(b)
	})
}
