package wire

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

// Cipher modes and nonce|ciphertext|tag framing match PingTunnel's public wire
// contract. Application keys are independently generated, not copied or shared.
func NormalizeEncryption(mode string) (string, error) {
	switch mode {
	case "", "none":
		return "none", nil
	case "aes128", "aes-128":
		return "aes128", nil
	case "aes256", "aes-256":
		return "aes256", nil
	case "chacha20":
		return "chacha20", nil
	default:
		return "", errors.New("unsupported encryption mode")
	}
}
func EncryptionKeySize(mode string) int {
	switch mode {
	case "aes128":
		return 16
	case "aes256", "chacha20":
		return 32
	default:
		return 0
	}
}

type Credential struct {
	Key          int32
	Mode, Secret string
}
type packetCipher struct {
	Credential
	aead cipher.AEAD
}
type cipherTable struct {
	byKey     map[int32]*packetCipher
	encrypted []*packetCipher
}
type peerCryptoKey struct {
	Source string
	EchoID uint16
}
type peerCryptoCache struct {
	candidates []*packetCipher
	at         time.Time
}

// Encrypted legacy envelopes contain no key ID. A bounded cold lookup discovers
// the credential; authenticated peers then use a small cached cipher list.
type Security struct {
	table  atomic.Pointer[cipherTable]
	mu     sync.Mutex
	peers  map[peerCryptoKey]peerCryptoCache
	tokens float64
	last   time.Time
}

func NewSecurity() *Security {
	s := &Security{peers: map[peerCryptoKey]peerCryptoCache{}, tokens: 64, last: time.Now()}
	s.table.Store(&cipherTable{byKey: map[int32]*packetCipher{}})
	return s
}
func buildCipher(c Credential) (*packetCipher, error) {
	mode, err := NormalizeEncryption(c.Mode)
	if err != nil {
		return nil, err
	}
	c.Mode = mode
	p := &packetCipher{Credential: c}
	if mode == "none" {
		return p, nil
	}
	key, err := base64.StdEncoding.DecodeString(c.Secret)
	if err != nil || len(key) != EncryptionKeySize(mode) {
		return nil, errors.New("invalid encryption key")
	}
	if mode == "chacha20" {
		p.aead, err = chacha20poly1305.New(key)
	} else {
		block, e := aes.NewCipher(key)
		if e != nil {
			return nil, e
		}
		p.aead, err = cipher.NewGCM(block)
	}
	return p, err
}
func (s *Security) SetCredentials(credentials []Credential) error {
	old := s.table.Load()
	next := &cipherTable{byKey: make(map[int32]*packetCipher, len(credentials))}
	for _, c := range credentials {
		mode, err := NormalizeEncryption(c.Mode)
		if err != nil {
			return err
		}
		c.Mode = mode
		if c.Key <= 0 || next.byKey[c.Key] != nil {
			return errors.New("invalid duplicated credential")
		}
		p := old.byKey[c.Key]
		if p == nil || p.Credential != c {
			p, err = buildCipher(c)
			if err != nil {
				return err
			}
		}
		next.byKey[c.Key] = p
		if p.aead != nil {
			next.encrypted = append(next.encrypted, p)
		}
	}
	s.table.Store(next)
	return nil
}
func (p *packetCipher) open(b []byte) (Message, error) {
	n := p.aead.NonceSize()
	if len(b) < n+p.aead.Overhead() || len(b) > MaxMessage+n+p.aead.Overhead() {
		return Message{}, ErrMalformed
	}
	plain, err := p.aead.Open(nil, b[:n], b[n:], nil)
	if err != nil {
		return Message{}, ErrMalformed
	}
	m, err := DecodeMessage(plain)
	if err != nil || m.Key != p.Key {
		return Message{}, ErrMalformed
	}
	return m, nil
}
func (s *Security) Decode(source string, id uint16, b []byte) (Message, string, error) {
	table := s.table.Load()
	if len(table.encrypted) == 0 {
		m, err := DecodeMessage(b)
		if err == nil {
			if p := table.byKey[m.Key]; p != nil && p.Mode == "none" {
				return m, "none", nil
			}
		}
		return Message{}, "", ErrMalformed
	}
	peer := peerCryptoKey{source, id}
	s.mu.Lock()
	cached := s.peers[peer]
	s.mu.Unlock()
	for _, c := range cached.candidates {
		if table.byKey[c.Key] != c {
			continue
		}
		if m, err := c.open(b); err == nil {
			if time.Since(cached.at) > 15*time.Second {
				s.mu.Lock()
				v := s.peers[peer]
				v.at = time.Now()
				s.peers[peer] = v
				s.mu.Unlock()
			}
			return m, c.Mode, nil
		}
	}
	if m, err := DecodeMessage(b); err == nil {
		c := table.byKey[m.Key]
		if c != nil && c.Mode == "none" {
			return m, "none", nil
		}
		return Message{}, "", ErrMalformed
	}
	if len(table.encrypted) == 0 || len(b) < 28 {
		return Message{}, "", ErrMalformed
	}
	now := time.Now()
	s.mu.Lock()
	s.tokens = min(64, s.tokens+now.Sub(s.last).Seconds()*32)
	s.last = now
	if s.tokens < 1 {
		s.mu.Unlock()
		return Message{}, "", ErrMalformed
	}
	s.tokens--
	s.mu.Unlock()
	for _, c := range table.encrypted {
		if m, err := c.open(b); err == nil {
			s.mu.Lock()
			if len(s.peers) >= 4096 {
				for k, v := range s.peers {
					if now.Sub(v.at) > time.Minute {
						delete(s.peers, k)
					}
				}
			}
			if len(s.peers) < 4096 || len(cached.candidates) > 0 {
				v := s.peers[peer]
				v.at = now
				found := false
				for _, p := range v.candidates {
					if p == c {
						found = true
					}
				}
				if !found {
					v.candidates = append([]*packetCipher{c}, v.candidates...)
					if len(v.candidates) > 4 {
						v.candidates = v.candidates[:4]
					}
				}
				s.peers[peer] = v
			}
			s.mu.Unlock()
			return m, c.Mode, nil
		}
	}
	return Message{}, "", ErrMalformed
}
func (s *Security) Encode(m Message) ([]byte, error) {
	p := s.table.Load().byKey[m.Key]
	if p == nil {
		return nil, errors.New("credential removed")
	}
	data := EncodeMessage(m)
	if p.aead == nil {
		return data, nil
	}
	nonce := make([]byte, p.aead.NonceSize(), p.aead.NonceSize()+len(data)+p.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce generation: %w", err)
	}
	return p.aead.Seal(nonce, nonce, data, nil), nil
}
