package panel

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"namedicmp/internal/core"
	"namedicmp/internal/store"
)

//go:embed web/*
var assets embed.FS

type Session struct {
	CSRF    string
	Expires time.Time
}
type attempt struct {
	count int
	until time.Time
}
type Panel struct {
	Store      *store.Store
	Core       *core.Core
	Config     store.Config
	Stats      *Stats
	mu         sync.Mutex
	sessions   map[string]Session
	attempts   map[string]attempt
	loginSlots chan struct{}
	addrCache map[string][]string
	addrAt time.Time
	addrBusy,addrLookup bool
}

func New(s *store.Store, c *core.Core, conf store.Config) *Panel {
	return &Panel{Store: s, Core: c, Config: conf, Stats: NewStats(conf.StatePath), sessions: map[string]Session{}, attempts: map[string]attempt{}, loginSlots: make(chan struct{}, 4)}
}
func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	jsonReply(w, status, map[string]string{"error": msg})
}
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("JSON required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return err == nil && u.Host == r.Host && u.Scheme == scheme && u.Path == ""
}
func (p *Panel) session(r *http.Request) (string, Session, bool) {
	cookie, err := r.Cookie("namedicmp_session")
	if err != nil {
		return "", Session{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.sessions[cookie.Value]
	if !ok || time.Now().After(s.Expires) {
		delete(p.sessions, cookie.Value)
		return "", Session{}, false
	}
	return cookie.Value, s, true
}
func (p *Panel) ClearSessions() { p.mu.Lock(); defer p.mu.Unlock(); p.sessions = map[string]Session{} }
func (p *Panel) ChangeAdmin(username, password string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.Store.SetAdmin(username, password); err != nil {
		return err
	}
	p.sessions = map[string]Session{}
	return nil
}
func (p *Panel) Handler() http.Handler {
	sub, _ := fs.Sub(assets, "web")
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			p.api(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			fail(w, 405, "method not allowed")
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/app.js" && r.URL.Path != "/app.css" && r.URL.Path != "/brand.svg" && r.URL.Path != "/fonts/manrope.woff2" && r.URL.Path != "/fonts/vazirmatn.woff2" {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
func (p *Panel) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	if !sameOrigin(r) {
		fail(w, 403, "origin rejected")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	now := time.Now()
	p.mu.Lock()
	for k, a := range p.attempts {
		if now.After(a.until) {
			delete(p.attempts, k)
		}
	}
	a := p.attempts[ip]
	blocked := a.count >= 5 || len(p.attempts) >= 4096
	p.mu.Unlock()
	if blocked {
		fail(w, 429, "try again in 15 minutes")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if readJSON(w, r, &input) != nil || len(input.Password) > 256 || len(input.Username) > 64 {
		fail(w, 400, "invalid login request")
		return
	}
	select {
	case p.loginSlots <- struct{}{}:
		defer func() { <-p.loginSlots }()
	default:
		fail(w, 429, "login busy; retry")
		return
	}
	credentialStamp := p.Store.AdminStamp()
	if !p.Store.CheckAdmin(input.Username, input.Password) {
		p.mu.Lock()
		a = p.attempts[ip]
		a.count++
		a.until = now.Add(15 * time.Minute)
		p.attempts[ip] = a
		p.mu.Unlock()
		fail(w, 401, "invalid username or password")
		return
	}
	token, csrf := store.ID()+store.ID(), store.ID()+store.ID()
	p.mu.Lock()
	if p.Store.AdminStamp() != credentialStamp {
		p.mu.Unlock()
		fail(w, 401, "credentials changed; retry login")
		return
	}
	delete(p.attempts, ip)
	for k, s := range p.sessions {
		if now.After(s.Expires) {
			delete(p.sessions, k)
		}
	}
	if len(p.sessions) >= 128 {
		p.mu.Unlock()
		fail(w, 429, "session limit reached")
		return
	}
	p.sessions[token] = Session{CSRF: csrf, Expires: now.Add(8 * time.Hour)}
	p.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "namedicmp_session", Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: 8 * 3600})
	jsonReply(w, 200, map[string]string{"csrf": csrf})
}
func (p *Panel) api(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/login" {
		p.login(w, r)
		return
	}
	token, s, ok := p.session(r)
	if !ok {
		fail(w, 401, "login required")
		return
	}
	if r.Method != "GET" {
		if !sameOrigin(r) || r.Header.Get("X-CSRF-Token") != s.CSRF {
			fail(w, 403, "CSRF rejected")
			return
		}
	}
	switch r.URL.Path {
	case "/api/session":
		if r.Method != "GET" {
			fail(w, 405, "method not allowed")
			return
		}
		jsonReply(w, 200, map[string]any{"csrf": s.CSRF, "public_host": p.connectionInfo()["effective_host"], "connection":p.connectionInfo(),"tls": r.TLS != nil, "version": "0.4.0", "protocol": "ICMPv4 / ICMPv6 TCP + UDP"})
	case "/api/logout":
		if r.Method != "POST" {
			fail(w, 405, "method not allowed")
			return
		}
		p.mu.Lock()
		delete(p.sessions, token)
		p.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "namedicmp_session", Value: "", Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		jsonReply(w, 200, map[string]bool{"ok": true})
	case "/api/home":
		if r.Method != "GET" {
			fail(w, 405, "method not allowed")
			return
		}
		m := p.Stats.Get()
		if p.Core != nil {
			m["core"] = p.Core.Metrics()
		}
		m["users"] = p.Store.UserCount()
		m["connection"]=p.connectionInfo()
		jsonReply(w, 200, m)
	case "/api/settings/connection":
		switch r.Method {
		case "GET":jsonReply(w,200,p.connectionInfo())
		case "PUT":
		 var input store.ConnectionSettings
		 if readJSON(w,r,&input)!=nil{fail(w,400,"invalid connection settings");return}
		 if input.Family=="ipv6"&&p.Core!=nil{available:=false;for _,family:=range p.Core.Families(){if family==6{available=true}};if !available{fail(w,400,"ICMPv6 socket unavailable; enable IPv6 on the VPS and restart the service first");return}}
		 if err:=p.Store.SetConnection(input);err!=nil{fail(w,400,err.Error());return}
		 jsonReply(w,200,p.connectionInfo())
		default:fail(w,405,"method not allowed")
		}
	case "/api/users":
		p.users(w, r)
	case "/api/outbounds":
		p.outbounds(w, r)
	default:
		fail(w, 404, "not found")
	}
}

type userInput struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Enabled    bool    `json:"enabled"`
	Expires    int64   `json:"expires"`
	Quota      int64   `json:"quota"`
	Outbound   string  `json:"outbound"`
	Encryption *string `json:"encryption"`
}

// Keep the private routing ID out of encryption-only client profiles.
func userProfile(u store.User) map[string]any {
	profile := map[string]any{
		"id": u.ID, "name": u.Name, "enabled": u.Enabled,
		"expires": u.Expires, "quota": u.Quota, "upload": u.Upload,
		"download": u.Download, "recovery": u.Recovery, "used": u.Used(), "outbound": u.Outbound,
		"created": u.Created, "encryption": u.Encryption,
	}
	if u.Encryption == "none" || u.Encryption == "" {
		profile["key"] = u.Key
	} else {
		profile["encryption_key"] = u.EncryptionKey
	}
	return profile
}

func (p *Panel) users(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		users := p.Store.Users()
		sort.Slice(users, func(i, j int) bool { return users[i].Created > users[j].Created })
		counts := map[string]int{}
		if p.Core != nil {
			counts = p.Core.Counts()
		}
		out := make([]map[string]any, 0, len(users))
		for _, u := range users {
			profile := userProfile(u)
			profile["used"] = u.Used()
			profile["online"] = counts[u.ID]
			profile["active"] = u.Allowed(time.Now())
			out = append(out, profile)
		}
		jsonReply(w, 200, out)
	case "POST", "PUT":
		var v userInput
		if err := readJSON(w, r, &v); err != nil {
			fail(w, 400, "invalid user request")
			return
		}
		if r.Method == "POST" {
			v.ID = ""
		} else if _, ok := p.Store.GetUser(v.ID); !ok {
			fail(w, 404, "user not found")
			return
		}
		old, _ := p.Store.GetUser(v.ID)
		mode := old.Encryption
		if v.Encryption != nil {
			mode = *v.Encryption
		}
		u, err := p.Store.PutUser(store.User{ID: v.ID, Name: v.Name, Enabled: v.Enabled, Expires: v.Expires, Quota: v.Quota, Outbound: v.Outbound, Encryption: mode})
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		if p.Core != nil {
			if err := p.Core.RefreshCredentials(); err != nil {
				fail(w, 500, "unable to load encryption settings")
				return
			}
			if !u.Allowed(time.Now()) || old.ID != "" && (old.Encryption != u.Encryption || old.EncryptionKey != u.EncryptionKey) {
				p.Core.Revoke(u.ID)
			}
		}
		jsonReply(w, 200, userProfile(u))
	case "DELETE":
		var v struct {
			ID string `json:"id"`
		}
		if readJSON(w, r, &v) != nil {
			fail(w, 400, "invalid request")
			return
		}
		if err := p.Store.DeleteUser(v.ID); err != nil {
			fail(w, 400, err.Error())
			return
		}
		if p.Core != nil {
			p.Core.Revoke(v.ID)
			if err := p.Core.RefreshCredentials(); err != nil {
				fail(w, 500, "unable to update credentials")
				return
			}
		}
		jsonReply(w, 200, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}
func (p *Panel) outbounds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		out := p.Store.Outbounds()
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		for i := range out {
			out[i].Password = ""
		}
		if out == nil {
			out = []store.Outbound{}
		}
		jsonReply(w, 200, out)
	case "POST", "PUT":
		var o store.Outbound
		if readJSON(w, r, &o) != nil {
			fail(w, 400, "invalid outbound request")
			return
		}
		if r.Method == "POST" {
			o.ID = ""
		}
		if r.Method == "PUT" {
			if _, ok := p.Store.GetOutbound(o.ID); !ok || o.ID == "" {
				fail(w, 404, "outbound not found")
				return
			}
		}
		o, err := p.Store.PutOutbound(o)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		o.Password = ""
		jsonReply(w, 200, o)
	case "DELETE":
		var v struct {
			ID string `json:"id"`
		}
		if readJSON(w, r, &v) != nil {
			fail(w, 400, "invalid request")
			return
		}
		if err := p.Store.DeleteOutbound(v.ID); err != nil {
			fail(w, 400, err.Error())
			return
		}
		jsonReply(w, 200, map[string]bool{"ok": true})
	default:
		fail(w, 405, "method not allowed")
	}
}
func (p *Panel) RunStats(ctx context.Context) { p.enableAddressLookup();p.Stats.Run(ctx) }

