package store

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"namedicmp/internal/wire"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var ErrDenied = errors.New("user disabled, expired or quota exhausted")

const LeaseBytes int64 = 256 * 1024
const iterations = 210000

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Key is the numeric credential in plaintext mode and a private routing ID
	// in encrypted mode. Encrypted wire packets use zero and authenticate by AEAD.
	Key           int32  `json:"key"`
	Enabled       bool   `json:"enabled"`
	Expires       int64  `json:"expires"`
	Quota         int64  `json:"quota"`
	Upload        int64  `json:"upload"`
	Download      int64  `json:"download"`
	Recovery      int64  `json:"recovery"`
	Reserved      int64  `json:"reserved"`
	Outbound      string `json:"outbound"`
	Created       int64  `json:"created"`
	Encryption    string `json:"encryption"`
	EncryptionKey string `json:"encryption_key,omitempty"`
}

func (u User) Used() int64 { return u.Upload + u.Download + u.Recovery }
func (u User) Allowed(now time.Time) bool {
	return u.Enabled && (u.Expires == 0 || now.Unix() < u.Expires) && (u.Quota == 0 || u.Used() < u.Quota)
}

type Outbound struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}
type Admin struct {
	Username string `json:"username"`
	Salt     string `json:"salt"`
	Hash     string `json:"hash"`
}
type disk struct {
	Version   int                 `json:"version"`
	Users     map[string]User     `json:"users"`
	Outbounds map[string]Outbound `json:"outbounds"`
	Admin     Admin               `json:"admin"`
	Connection *ConnectionSettings `json:"connection,omitempty"`
}
type Store struct {
	mu    sync.Mutex
	path  string
	lock  *os.File
	d     disk
	byKey map[int32]string
	fault error
	dirty bool
}

// UserCount avoids copying every user and credential for dashboard polling.
func (s *Store) UserCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.d.Users)
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("state already in use: %w", err)
	}
	s := &Store{path: path, lock: lock, d: disk{Version: 1, Users: map[string]User{}, Outbounds: map[string]Outbound{}}, byKey: map[int32]string{}}
	b, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(b, &s.d)
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		lock.Close()
		return nil, err
	}
	if s.d.Version != 1 || s.d.Users == nil || s.d.Outbounds == nil {
		lock.Close()
		return nil, errors.New("unsupported state format")
	}
	for id, u := range s.d.Users {
		if u.Key <= 0 || s.byKey[u.Key] != "" {
			lock.Close()
			return nil, errors.New("invalid or duplicated key")
		}
		mode, e := wire.NormalizeEncryption(u.Encryption)
		if e != nil {
			lock.Close()
			return nil, e
		}
		u.Encryption = mode
		if mode != "none" {
			key, e := base64.StdEncoding.DecodeString(u.EncryptionKey)
			if e != nil || len(key) != wire.EncryptionKeySize(mode) {
				lock.Close()
				return nil, errors.New("invalid stored encryption key")
			}
		}
		u.Recovery += u.Reserved
		u.Reserved = 0
		s.d.Users[id] = u
		s.byKey[u.Key] = id
	}
	if err = s.save(); err != nil {
		lock.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) save() error {
	b, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, s.path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	err = dir.Sync()
	if err == nil {
		s.dirty = false
	}
	return err
}
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fault != nil {
		return s.fault
	}
	if !s.dirty {
		return nil
	}
	err := s.save()
	if err != nil {
		s.fault = err
	}
	return err
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, u := range s.d.Users {
		u.Reserved = 0
		s.d.Users[id] = u
	}
	err := s.save()
	syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
	s.lock.Close()
	return err
}
func (s *Store) Fault() error { s.mu.Lock(); defer s.mu.Unlock(); return s.fault }
func ID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func (s *Store) Users() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]User, 0, len(s.d.Users))
	for _, u := range s.d.Users {
		u.Reserved = 0
		out = append(out, u)
	}
	return out
}
func (s *Store) Lookup(key int32) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.d.Users[s.byKey[key]]
	return u, ok && s.fault == nil
}
func (s *Store) GetUser(id string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.d.Users[id]
	return u, ok
}
func (s *Store) PutUser(u User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fault != nil {
		return u, s.fault
	}
	u.Name = strings.TrimSpace(u.Name)
	if len(u.Name) < 1 || len(u.Name) > 100 || u.Quota < 0 || u.Quota > 1<<60 || u.Expires < 0 {
		return u, errors.New("invalid name, quota or expiry")
	}
	if u.Outbound != "" {
		if _, ok := s.d.Outbounds[u.Outbound]; !ok {
			return u, errors.New("outbound does not exist")
		}
	}
	old, exists := s.d.Users[u.ID]
	mode, err := wire.NormalizeEncryption(u.Encryption)
	if err != nil {
		return u, err
	}
	u.Encryption = mode
	if mode == "none" {
		u.EncryptionKey = ""
	} else if exists && old.Encryption == mode && old.EncryptionKey != "" {
		u.EncryptionKey = old.EncryptionKey
	} else {
		key := make([]byte, wire.EncryptionKeySize(mode))
		if _, err := rand.Read(key); err != nil {
			return u, err
		}
		u.EncryptionKey = base64.StdEncoding.EncodeToString(key)
	}
	if exists {
		u.Key = old.Key
		u.Upload = old.Upload
		u.Download = old.Download
		u.Recovery = old.Recovery
		u.Reserved = old.Reserved
		u.Created = old.Created
		if u.Quota > 0 {
			u.Reserved = min(u.Reserved, max(int64(0), u.Quota-u.Used()))
		}
	} else {
		if len(s.d.Users) >= 10000 {
			return u, errors.New("user limit reached")
		}
		u.ID = ID()
		u.Created = time.Now().Unix()
		u.Upload = 0
		u.Download = 0
		u.Recovery = 0
		u.Reserved = 0
		for {
			var b [4]byte
			if _, err := rand.Read(b[:]); err != nil {
				return u, err
			}
			u.Key = int32(binary.BigEndian.Uint32(b[:]) & 0x7fffffff)
			if u.Key > 0 && s.byKey[u.Key] == "" {
				break
			}
		}
	}
	s.d.Users[u.ID] = u
	if err := s.save(); err != nil {
		if exists {
			s.d.Users[u.ID] = old
		} else {
			delete(s.d.Users, u.ID)
		}
		s.fault = err
		return u, err
	}
	s.byKey[u.Key] = u.ID
	return u, nil
}
func (s *Store) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.d.Users[id]
	if !ok {
		return errors.New("user does not exist")
	}
	delete(s.d.Users, id)
	if err := s.save(); err != nil {
		s.d.Users[id] = u
		s.fault = err
		return err
	}
	delete(s.byKey, u.Key)
	return nil
}

// Charge counts unique application payload once, before forwarding. Retransmits
// never pass this call twice. Durable credit is acquired in blocks, not packets.
func (s *Store) Charge(id string, n int, upload bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.d.Users[id]
	if !ok || s.fault != nil || !u.Allowed(time.Now()) || n < 0 {
		return ErrDenied
	}
	if u.Quota > 0 && int64(n) > u.Quota-u.Used() {
		return ErrDenied
	}
	if u.Reserved < int64(n) {
		old := u
		grant := LeaseBytes
		if int64(n) > grant {
			grant = int64(n)
		}
		if u.Quota > 0 && grant > u.Quota-u.Used()-u.Reserved {
			grant = u.Quota - u.Used() - u.Reserved
		}
		u.Reserved += grant
		s.d.Users[id] = u
		if err := s.save(); err != nil {
			s.d.Users[id] = old
			s.fault = err
			return fmt.Errorf("usage persistence: %w", err)
		}
	}
	u.Reserved -= int64(n)
	if upload {
		u.Upload += int64(n)
	} else {
		u.Download += int64(n)
	}
	s.d.Users[id] = u
	s.dirty = true
	return nil
}
func (s *Store) Outbounds() []Outbound {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Outbound
	for _, o := range s.d.Outbounds {
		out = append(out, o)
	}
	return out
}
func (s *Store) GetOutbound(id string) (Outbound, bool) {
	if id == "" {
		return Outbound{Kind: "direct"}, true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.d.Outbounds[id]
	return o, ok
}
func (s *Store) PutOutbound(o Outbound) (Outbound, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fault != nil {
		return o, s.fault
	}
	o.Name = strings.TrimSpace(o.Name)
	if len(o.Name) == 0 || len(o.Name) > 100 || (o.Kind != "socks5" && o.Kind != "http") || o.Host == "" || len(o.Host) > 253 || strings.ContainsAny(o.Host, "\r\n /\\") || o.Port < 1 || o.Port > 65535 || len(o.Username) > 255 || len(o.Password) > 255 || strings.ContainsAny(o.Username+o.Password, "\r\n") {
		return o, errors.New("invalid outbound")
	}
	old, exists := s.d.Outbounds[o.ID]
	if !exists {
		if len(s.d.Outbounds) >= 100 {
			return o, errors.New("outbound limit reached")
		}
		o.ID = ID()
	} else if o.Password == "" {
		o.Password = old.Password
	}
	s.d.Outbounds[o.ID] = o
	if err := s.save(); err != nil {
		if exists {
			s.d.Outbounds[o.ID] = old
		} else {
			delete(s.d.Outbounds, o.ID)
		}
		s.fault = err
		return o, err
	}
	return o, nil
}
func (s *Store) DeleteOutbound(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.d.Users {
		if u.Outbound == id {
			return errors.New("outbound is assigned to a user")
		}
	}
	o, ok := s.d.Outbounds[id]
	if !ok {
		return errors.New("outbound does not exist")
	}
	delete(s.d.Outbounds, id)
	if err := s.save(); err != nil {
		s.d.Outbounds[id] = o
		s.fault = err
		return err
	}
	return nil
}
func passwordHash(password string, salt []byte) []byte {
	b, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		panic(err)
	}
	return b
}
func (s *Store) SetAdmin(username, password string) error {
	if len(username) < 1 || len(username) > 64 || len(password) < 12 || len(password) > 256 {
		return errors.New("username required; password must have 12–256 characters")
	}
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return err
	}
	a := Admin{Username: username, Salt: base64.StdEncoding.EncodeToString(salt[:]), Hash: base64.StdEncoding.EncodeToString(passwordHash(password, salt[:]))}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.d.Admin
	s.d.Admin = a
	if err := s.save(); err != nil {
		s.d.Admin = old
		return err
	}
	return nil
}
func (s *Store) HasAdmin() bool     { s.mu.Lock(); defer s.mu.Unlock(); return s.d.Admin.Hash != "" }
func (s *Store) AdminStamp() string { s.mu.Lock(); defer s.mu.Unlock(); return s.d.Admin.Hash }
func (s *Store) CheckAdmin(username, password string) bool {
	s.mu.Lock()
	a := s.d.Admin
	s.mu.Unlock()
	salt, err := base64.StdEncoding.DecodeString(a.Salt)
	if err != nil {
		return false
	}
	expected, err := base64.StdEncoding.DecodeString(a.Hash)
	if err != nil || len(expected) != 32 {
		return false
	}
	got := passwordHash(password, salt)
	return subtle.ConstantTimeCompare(got, expected) == 1 && subtle.ConstantTimeCompare([]byte(username), []byte(a.Username)) == 1
}

