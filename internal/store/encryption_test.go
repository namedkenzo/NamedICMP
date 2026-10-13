package store

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptionIsOptionalAndStable(t *testing.T) {
	s := testStore(t)
	u, err := s.PutUser(User{Name: "default", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if u.Encryption != "none" || u.EncryptionKey != "" {
		t.Fatal("default encryption enabled")
	}
	for _, mode := range []string{"aes128", "aes256", "chacha20"} {
		u.Encryption = mode
		u, err = s.PutUser(u)
		if err != nil {
			t.Fatal(err)
		}
		secret := u.EncryptionKey
		key, _ := base64.StdEncoding.DecodeString(secret)
		want := 32
		if mode == "aes128" {
			want = 16
		}
		if len(key) != want {
			t.Fatal("wrong key size")
		}
		u.Name = "updated"
		u, err = s.PutUser(u)
		if err != nil || u.EncryptionKey != secret {
			t.Fatal("unrelated edit rotates encryption key")
		}
	}
	u.Encryption = "none"
	u, _ = s.PutUser(u)
	if u.EncryptionKey != "" {
		t.Fatal("secret retained when encryption disabled")
	}
	u.Encryption = "unsupported"
	if _, err = s.PutUser(u); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

func TestLegacyStateAndConfigKeepUsersAndDefaultEncryptionOff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	legacy := `{"version":1,"users":{"legacy":{"id":"legacy","name":"existing","key":123456,"enabled":true,"quota":999999,"upload":100,"download":200,"recovery":3,"reserved":0,"outbound":"","created":123}},"outbounds":{},"admin":{"username":"admin","salt":"unchanged","hash":"unchanged"}}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u, ok := s.GetUser("legacy")
	if !ok || u.Key != 123456 || u.Used() != 303 || u.Quota != 999999 || u.Encryption != "none" || u.EncryptionKey != "" || s.d.Admin.Hash != "unchanged" {
		t.Fatal("legacy data changed during migration")
	}
	configPath := filepath.Join(dir, "config.json")
	config := DefaultConfig()
	data, _ := json.Marshal(config)
	var fields map[string]any
	json.Unmarshal(data, &fields)
	delete(fields, "max_udp_sessions_per_user")
	data, _ = json.Marshal(fields)
	os.WriteFile(configPath, data, 0600)
	loaded, err := LoadConfig(configPath)
	if err != nil || loaded.MaxUDPPerUser != 128 || loaded.MaxPerUser != 32 || loaded.PanelListen != config.PanelListen {
		t.Fatal("old configuration did not load with separate UDP default")
	}
}
func TestIdleFlushDoesNotRewriteState(t *testing.T) {
	s := testStore(t)
	before, err := os.Stat(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Flush(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(s.path)
	if !os.SameFile(before, after) {
		t.Fatal("idle flush replaced the file")
	}
	u, _ := s.PutUser(User{Name: "dirty", Enabled: true})
	s.Charge(u.ID, 1, true)
	if err = s.Flush(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(s.path)
	var snapshot disk
	if err = json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Users[u.ID].Upload != 1 {
		t.Fatal("usage was not persisted by checkpoint")
	}
}
