package panel

import (
	"bytes"
	"encoding/json"
	"io"
	"namedicmp/internal/store"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthCSRFAndUserLifecycle(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.SetAdmin("admin", "test-password-1234")
	p := New(s, nil, store.DefaultConfig())
	srv := httptest.NewServer(p.Handler())
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	request := func(path, method, body, csrf, origin string) (int, []byte) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, b
	}
	code, _ := request("/api/users", "GET", "", "", "")
	if code != 401 {
		t.Fatal("unauthenticated access")
	}
	code, _ = request("/api/login", "POST", `{"username":"admin","password":"test-password-1234"}`, "", "https://evil.example")
	if code != 403 {
		t.Fatal("cross origin login")
	}
	code, b := request("/api/login", "POST", `{"username":"admin","password":"test-password-1234"}`, "", srv.URL)
	if code != 200 {
		t.Fatalf("login %d %s", code, b)
	}
	var auth map[string]string
	json.Unmarshal(b, &auth)
	token := auth["csrf"]
	user := `{"name":"<script>alert(1)</script>","enabled":true,"expires":0,"quota":100,"outbound":""}`
	code, _ = request("/api/users", "POST", user, "", srv.URL)
	if code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	code, b = request("/api/users", "POST", user, token, srv.URL)
	if code != 200 {
		t.Fatalf("user create: %s", b)
	}
	var u store.User
	json.Unmarshal(b, &u)
	if u.Key <= 0 || u.Encryption != "none" || u.EncryptionKey != "" {
		t.Fatal("user key or default-off encryption")
	}
	edit := map[string]any{"id": u.ID, "name": u.Name, "enabled": true, "expires": 0, "quota": 100, "outbound": "", "encryption": "aes128"}
	encoded, _ := json.Marshal(edit)
	code, b = request("/api/users", "PUT", string(encoded), token, srv.URL)
	if code != 200 {
		t.Fatalf("encryption update: %s", b)
	}
	if bytes.Contains(b, []byte(`"key":`)) {
		t.Fatal("encrypted create/update response exposes numeric key")
	}
	json.Unmarshal(b, &u)
	secret := u.EncryptionKey
	if u.Encryption != "aes128" || secret == "" {
		t.Fatal("encryption key was not generated")
	}
	// Older API consumers can omit the new field without disabling encryption.
	delete(edit, "encryption")
	edit["quota"] = 200
	encoded, _ = json.Marshal(edit)
	code, b = request("/api/users", "PUT", string(encoded), token, srv.URL)
	json.Unmarshal(b, &u)
	if code != 200 || u.Encryption != "aes128" || u.EncryptionKey != secret || u.Quota != 200 {
		t.Fatal("unrelated API edit changed encryption")
	}
	code, b = request("/api/users", "GET", "", "", "")
	if code != 200 || !bytes.Contains(b, []byte(`\u003cscript\u003e`)) {
		t.Fatal("user JSON not escaped")
	}
	if bytes.Contains(b, []byte(`"key":`)) {
		t.Fatal("encrypted list exposes numeric key")
	}
	// Switching to numeric mode removes the encryption credential.
	edit["encryption"] = "none"
	encoded, _ = json.Marshal(edit)
	code, b = request("/api/users", "PUT", string(encoded), token, srv.URL)
	u = store.User{}
	json.Unmarshal(b, &u)
	if code != 200 || u.Key <= 0 || u.EncryptionKey != "" {
		t.Fatal("numeric mode retained encryption")
	}
	code, _ = request("/api/users", "DELETE", `{"id":"`+u.ID+`"}`, token, srv.URL)
	if code != 200 || len(s.Users()) != 0 {
		t.Fatal("delete user")
	}
	code, _ = request("/api/logout", "POST", `{}`, token, srv.URL)
	if code != 200 {
		t.Fatal("logout")
	}
	code, _ = request("/api/home", "GET", "", "", "")
	if code != 401 {
		t.Fatal("revoked session accepted")
	}
}
func TestLoginRateLimitAndStaticHeaders(t *testing.T) {
	s, _ := store.Open(filepath.Join(t.TempDir(), "state.json"))
	defer s.Close()
	s.SetAdmin("a", "test-password-1234")
	p := New(s, nil, store.DefaultConfig())
	h := p.Handler()
	for i := 0; i < 6; i++ {
		r := httptest.NewRequest("POST", "http://panel/api/login", strings.NewReader(`{"username":"a","password":"wrong"}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 401
		if i == 5 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://panel/", nil))
	if w.Code != 200 || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("static assets/security headers")
	}
}

func TestPublicProfilePreservesCountersAndExclusiveCredentials(t *testing.T) {
	u := store.User{Key: 123, Encryption: "none", Quota: 1 << 60, Upload: 1<<54 + 1}
	encoded, err := json.Marshal(userProfile(u))
	if err != nil || !bytes.Contains(encoded, []byte(`"upload":18014398509481985`)) {
		t.Fatal("profile rounded integer counters")
	}
	if _, ok := userProfile(u)["encryption_key"]; ok {
		t.Fatal("numeric profile has an encryption credential")
	}
	u.Encryption, u.EncryptionKey = "aes256", "secret"
	profile := userProfile(u)
	if _, ok := profile["key"]; ok {
		t.Fatal("encrypted profile has a numeric credential")
	}
	if profile["encryption_key"] != "secret" {
		t.Fatal("missing encryption credential")
	}
}
