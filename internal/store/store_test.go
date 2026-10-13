package store

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestIsolationAndQuota(t *testing.T) {
	s := testStore(t)
	a, err := s.PutUser(User{Name: "A", Enabled: true, Quota: 100})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.PutUser(User{Name: "B", Enabled: true, Quota: 100})
	if err != nil {
		t.Fatal(err)
	}
	if a.Key == b.Key {
		t.Fatal("duplicate credentials")
	}
	if err = s.Charge(a.ID, 60, true); err != nil {
		t.Fatal(err)
	}
	if err = s.Charge(a.ID, 41, false); err == nil {
		t.Fatal("quota overrun")
	}
	if err = s.Charge(b.ID, 7, false); err != nil {
		t.Fatal(err)
	}
	av, _ := s.GetUser(a.ID)
	bv, _ := s.GetUser(b.ID)
	if av.Used() != 60 || bv.Used() != 7 {
		t.Fatal("mixed counters")
	}
	a.Enabled = false
	a, err = s.PutUser(a)
	if err != nil {
		t.Fatal(err)
	}
	if s.Charge(a.ID, 1, true) == nil {
		t.Fatal("disabled user forwarded")
	}
	b.Expires = time.Now().Add(-time.Hour).Unix()
	s.PutUser(b)
	if s.Charge(b.ID, 1, true) == nil {
		t.Fatal("expired user forwarded")
	}
}
func TestConcurrentQuota(t *testing.T) {
	s := testStore(t)
	u, _ := s.PutUser(User{Name: "test", Enabled: true, Quota: 1000})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = s.Charge(u.ID, 1, true)
			}
		}()
	}
	wg.Wait()
	u, _ = s.GetUser(u.ID)
	if u.Used() != 1000 {
		t.Fatalf("concurrent quota = %d", u.Used())
	}
}
func TestCrashDoesNotRestoreQuota(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := s.PutUser(User{Name: "crash", Enabled: true, Quota: 1 << 20})
	if err = s.Charge(u.ID, 100, true); err != nil {
		t.Fatal(err)
	}
	s.lock.Close() // mimic abrupt exit: do not flush or refund unused credit
	recovered, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	got, _ := recovered.GetUser(u.ID)
	if got.Used() < 100 || got.Used() > LeaseBytes {
		t.Fatalf("unsafe crash recovery = %d", got.Used())
	}
}
func TestCleanRestartExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, _ := Open(path)
	u, _ := s.PutUser(User{Name: "clean", Enabled: true})
	s.Charge(u.ID, 123, true)
	s.Charge(u.ID, 456, false)
	s.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u, _ = s.GetUser(u.ID)
	if u.Upload != 123 || u.Download != 456 || u.Recovery != 0 {
		t.Fatal("clean shutdown inaccurate")
	}
}
func TestLockAndCredentials(t *testing.T) {
	s := testStore(t)
	if other, err := Open(s.path); err == nil {
		other.Close()
		t.Fatal("second writer accepted")
	}
	if err := s.SetAdmin("admin", "short"); err == nil {
		t.Fatal("short password")
	}
	if err := s.SetAdmin("admin", "test-password-1234"); err != nil {
		t.Fatal(err)
	}
	if !s.CheckAdmin("admin", "test-password-1234") || s.CheckAdmin("admin", "incorrect") || s.CheckAdmin("other", "test-password-1234") {
		t.Fatal("credential verification")
	}
}
func TestQuotaReductionClampsOutstandingCredit(t *testing.T) {
	s := testStore(t)
	u, _ := s.PutUser(User{Name: "resize", Enabled: true})
	s.Charge(u.ID, 100, true)
	u.Quota = 150
	u, err := s.PutUser(u)
	if err != nil {
		t.Fatal(err)
	}
	if u.Reserved > 50 {
		t.Fatal("outstanding credit exceeds reduced quota")
	}
	if err = s.Charge(u.ID, 50, false); err != nil {
		t.Fatal(err)
	}
	if s.Charge(u.ID, 1, true) == nil {
		t.Fatal("reduced quota overrun")
	}
}
func TestPersistenceFaultFailsClosed(t *testing.T) {
	s := testStore(t)
	u, _ := s.PutUser(User{Name: "test", Enabled: true})
	s.path = filepath.Join(t.TempDir(), "missing", "state.json")
	if s.Charge(u.ID, 1, true) == nil {
		t.Fatal("forwarded without durable credit")
	}
	if _, ok := s.Lookup(u.Key); ok {
		t.Fatal("fault not fail closed")
	}
}
