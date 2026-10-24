package store

import (
	"path/filepath"
	"testing"
)

func TestUserCountFollowsState(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil { t.Fatal(err) }
	defer s.Close()
	if s.UserCount() != 0 { t.Fatal("empty store count") }
	if _, err = s.PutUser(User{Name: "count", Enabled: true}); err != nil { t.Fatal(err) }
	if s.UserCount() != 1 { t.Fatal("user not counted") }
}
