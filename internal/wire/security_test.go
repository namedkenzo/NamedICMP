package wire

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestAEADIndependentVectors(t *testing.T) {
	vectors := map[string]string{"aes128": "000102030405060708090a0bfd0dcaab0252b4191bf202f84fd3046793527f952705a7f96910eedb5ada3b5f7dc0f08c42", "aes256": "000102030405060708090a0b2963bb7ea1ac8156dd61f4f9c8990c02a3a2e247847bf6facca7c1c1ecb1f989204f71c51c", "chacha20": "000102030405060708090a0be79a65654d5ee60de7a35c81e16d7a0ce904d794250ff2344f70a94720e047a5e7f8c45129"}
	for mode, encoded := range vectors {
		key := make([]byte, EncryptionKeySize(mode))
		for i := range key {
			key[i] = byte(i)
		}
		p, err := buildCipher(Credential{Key: 1, Mode: mode, Secret: base64.StdEncoding.EncodeToString(key)})
		if err != nil {
			t.Fatal(err)
		}
		v, _ := hex.DecodeString(encoded)
		plain, err := p.aead.Open(nil, v[:12], v[12:], nil)
		if err != nil || string(plain) != "namedICMP crypto test" {
			t.Fatalf("%s vector mismatch: %v", mode, err)
		}
		v[15] ^= 1
		if _, err = p.aead.Open(nil, v[:12], v[12:], nil); err == nil {
			t.Fatal("modified ciphertext accepted")
		}
	}
}
func credential(key int32, mode string) Credential {
	return Credential{Key: key, Mode: mode, Secret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{byte(key)}, EncryptionKeySize(mode)))}
}
func TestMixedCiphersDefaultAndNoDowngrade(t *testing.T) {
	s := NewSecurity()
	users := []Credential{credential(1, "none"), credential(2, "aes128"), credential(3, "aes256"), credential(4, "chacha20")}
	if err := s.SetCredentials(users); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 3; n++ {
		for _, u := range users {
			m := Message{Connection: "connection", Target: "8.8.8.8:53", Key: u.Key, Payload: []byte("data")}
			b, err := s.Encode(m)
			if err != nil {
				t.Fatal(err)
			}
			decoded, mode, err := s.Decode("192.0.2.1", 7, b)
			if err != nil || mode != u.Mode || !bytes.Equal(decoded.Payload, m.Payload) {
				t.Fatalf("%s routing: %v", u.Mode, err)
			}
			if u.Mode != "none" {
				if _, _, err = s.Decode("192.0.2.1", 7, EncodeMessage(m)); err == nil {
					t.Fatal("plaintext downgrade accepted")
				}
			}
		}
	}
	old, _ := s.Encode(Message{Key: 2, Payload: []byte("old")})
	s.SetCredentials([]Credential{users[0]})
	if _, _, err := s.Decode("192.0.2.1", 7, old); err == nil {
		t.Fatal("removed cached key accepted")
	}
}
func TestWrongEncryptionKeyDenied(t *testing.T) {
	server, client := NewSecurity(), NewSecurity()
	a := credential(1, "aes256")
	b := a
	b.Secret = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	server.SetCredentials([]Credential{a})
	client.SetCredentials([]Credential{b})
	packet, _ := client.Encode(Message{Key: 1, Payload: []byte("test")})
	if _, _, err := server.Decode("192.0.2.1", 1, packet); err == nil {
		t.Fatal("wrong cipher key authenticated")
	}
}
func FuzzEncryptedDecoder(f *testing.F) {
	s := NewSecurity()
	credentials := []Credential{credential(1, "none"), credential(2, "aes128"), credential(3, "aes256"), credential(4, "chacha20")}
	s.SetCredentials(credentials)
	f.Add([]byte("bad packet"))
	for _, c := range credentials {
		wireKey := c.Key
		if c.Mode != "none" {
			wireKey = 0
		}
		data := EncodeMessage(Message{Key: wireKey, Connection: "seed", Target: "8.8.8.8:53", Payload: []byte("seed")})
		if c.Mode != "none" {
			p, _ := buildCipher(c)
			nonce := make([]byte, 12)
			data = p.aead.Seal(nonce, nonce, data, nil)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, b []byte) { _, _, _ = s.Decode("192.0.2.1", 4, b) })
}

// Build client packets directly: these must work with no numeric password and
// responses must also carry the protocol default zero, even for multiple users.
func TestEncryptionOnlyCredentials(t *testing.T) {
	for _, mode := range []string{"aes128", "aes256", "chacha20"} {
		t.Run(mode, func(t *testing.T) {
			server := NewSecurity()
			a, b := credential(123, mode), credential(456, mode)
			if err := server.SetCredentials([]Credential{a, b, credential(789, "none")}); err != nil {
				t.Fatal(err)
			}
			for _, user := range []Credential{a, b} {
				cipher, _ := buildCipher(user)
				packet := func(key int32) []byte {
					nonce := make([]byte, cipher.aead.NonceSize())
					return cipher.aead.Seal(nonce, nonce, EncodeMessage(Message{Key: key, Kind: 1, Payload: []byte("probe")}), nil)
				}
				for _, source := range []string{"cold-peer", "cold-peer"} {
					m, gotMode, err := server.Decode(source, 7, packet(0))
					if err != nil || m.Key != user.Key || gotMode != mode {
						t.Fatalf("encryption-only routing: %+v %s %v", m, gotMode, err)
					}
					reply, err := server.Encode(m)
					if err != nil {
						t.Fatal(err)
					}
					n := cipher.aead.NonceSize()
					plain, err := cipher.aead.Open(nil, reply[:n], reply[n:], nil)
					if err != nil {
						t.Fatal(err)
					}
					response, err := DecodeMessage(plain)
					if err != nil || response.Key != 0 || !bytes.Equal(response.Payload, m.Payload) {
						t.Fatal("numeric routing key leaked onto encrypted wire")
					}
				}
				if _, _, err := server.Decode("cold-peer", 7, packet(user.Key)); err == nil {
					t.Fatal("combined encryption and numeric password accepted")
				}
				if _, _, err := server.Decode("cold-peer", 7, EncodeMessage(Message{Key: user.Key})); err == nil {
					t.Fatal("encrypted user accepted numeric-only authentication")
				}
			}
		})
	}
}
