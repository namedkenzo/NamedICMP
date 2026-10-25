package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"namedicmp/internal/core"
	"namedicmp/internal/panel"
	"namedicmp/internal/store"
)

const version = "0.3.1"

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func inputCredentials() (credentials, error) {
	var v credentials
	d := json.NewDecoder(io.LimitReader(os.Stdin, 4096))
	d.DisallowUnknownFields()
	err := d.Decode(&v)
	return v, err
}
func main() {
	if len(os.Args) < 2 {
		fmt.Println("namedICMP", version, "\nCommands: serve, init, admin, diagnostics, version")
		return
	}
	var err error
	switch os.Args[1] {
	case "version":
		fmt.Println(version)
		return
	case "init":
		err = initialize(os.Args[2:])
	case "admin":
		err = adminClient(os.Args[2:])
	case "diagnostics":
		err = diagnostics(os.Args[2:])
	case "serve":
		err = serve(os.Args[2:])
	default:
		err = errors.New("unknown command")
	}
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func initialize(args []string) error {
	f := flag.NewFlagSet("init", flag.ContinueOnError)
	path := f.String("config", "/etc/namedicmp/config.json", "configuration path")
	listen := f.String("listen", "0.0.0.0:8443", "panel bind address")
	state := f.String("state", "/var/lib/namedicmp/state.json", "state path")
	host := f.String("public-host", "", "public IP or domain for exports")
	if err := f.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*path); !errors.Is(err, os.ErrNotExist) {
		return errors.New("configuration already exists; use admin command for credentials")
	}
	v, err := inputCredentials()
	if err != nil {
		return err
	}
	s, err := store.Open(*state)
	if err != nil {
		return err
	}
	defer s.Close()
	if err = s.SetAdmin(v.Username, v.Password); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(*path), 0700); err != nil {
		return err
	}
	c := store.DefaultConfig()
	c.PanelListen = *listen
	c.StatePath = *state
	c.PublicHost = *host
	if err = store.SaveConfig(*path, c); err != nil {
		return err
	}
	_, err = store.LoadConfig(*path)
	return err
}
func adminClient(args []string) error {
	f := flag.NewFlagSet("admin", flag.ContinueOnError)
	sock := f.String("socket", "/run/namedicmp/admin.sock", "local management socket")
	if err := f.Parse(args); err != nil {
		return err
	}
	v, err := inputCredentials()
	if err != nil {
		return err
	}
	b, _ := json.Marshal(v)
	client := http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", *sock)
	}}}
	res, err := client.Post("http://unix/admin", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("credential change: %s", body)
	}
	fmt.Println("Credentials changed; existing web sessions revoked. ICMP tunnels kept running.")
	return nil
}

func serve(args []string) error {
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	path := f.String("config", "/etc/namedicmp/config.json", "configuration path")
	panelOnly := f.Bool("panel-only", false, "development preview without ICMP")
	sock := f.String("admin-socket", "/run/namedicmp/admin.sock", "local management socket")
	if err := f.Parse(args); err != nil {
		return err
	}
	conf, err := store.LoadConfig(*path)
	if err != nil {
		return err
	}
	s, err := store.Open(conf.StatePath)
	if err != nil {
		return err
	}
	defer s.Close()
	if !s.HasAdmin() {
		return errors.New("initialize admin credentials first")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	var c *core.Core
	if !*panelOnly {
		raw, err := core.ListenRaw(conf.ICMPListen)
		if err != nil {
			return err
		}
		c, err = core.New(s, conf, raw)
		if err != nil {
			raw.Close()
			return err
		}
		defer c.Stop()
	}
	p := panel.New(s, c, conf)
	go p.RunStats(ctx)
	maintenanceDone := make(chan struct{})
	go func() {
		defer close(maintenanceDone)
		if c != nil {
			c.Maintain(ctx)
		} else {
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if err := s.Flush(); err != nil {
						log.Printf("state failure: %v", err)
					}
				}
			}
		}
	}()
	server := &http.Server{Addr: conf.PanelListen, Handler: p.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ln, err := net.Listen("tcp", conf.PanelListen)
	if err != nil {
		cancel()
		<-maintenanceDone
		return err
	}
	var certificate atomic.Pointer[tls.Certificate]
	reloadCert := func() error {
		cert, err := tls.LoadX509KeyPair(conf.TLSCert, conf.TLSKey)
		if err != nil {
			return err
		}
		certificate.Store(&cert)
		return nil
	}
	if conf.TLSCert != "" {
		if err = reloadCert(); err != nil {
			ln.Close()
			cancel()
			<-maintenanceDone
			return err
		}
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return certificate.Load(), nil }}
		ln = tls.NewListener(ln, server.TLSConfig)
	}
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hup:
				if conf.TLSCert != "" {
					if err := reloadCert(); err != nil {
						log.Printf("certificate reload rejected: %v", err)
					} else {
						log.Print("certificate reloaded")
					}
				}
			}
		}
	}()
	var admin *http.Server
	var adminLn net.Listener
	if *sock != "" {
		if err = os.MkdirAll(filepath.Dir(*sock), 0700); err != nil {
			ln.Close()
			cancel()
			<-maintenanceDone
			return err
		}
		_ = os.Remove(*sock)
		adminLn, err = net.Listen("unix", *sock)
		if err != nil {
			ln.Close()
			cancel()
			<-maintenanceDone
			return err
		}
		defer os.Remove(*sock)
		if err = os.Chmod(*sock, 0600); err != nil {
			adminLn.Close()
			ln.Close()
			cancel()
			<-maintenanceDone
			return err
		}
		admin = &http.Server{ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && r.URL.Path == "/diagnostics" {
				w.Header().Set("Content-Type", "application/json")
				health := map[string]any{"version": version, "storage_healthy": s.Fault() == nil, "core_enabled": c != nil}
				if c != nil {
					health["core"] = c.Metrics()
				}
				if s.Fault() != nil {
					w.WriteHeader(503)
				}
				_ = json.NewEncoder(w).Encode(health)
				return
			}
			if r.Method != "POST" || r.URL.Path != "/admin" {
				http.NotFound(w, r)
				return
			}
			var v credentials
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&v) != nil {
				http.Error(w, "invalid credentials", 400)
				return
			}
			if err := p.ChangeAdmin(v.Username, v.Password); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			w.WriteHeader(200)
		})}
	} else if !*panelOnly {
		ln.Close()
		cancel()
		<-maintenanceDone
		return errors.New("management socket may only be disabled in panel-only preview")
	}
	errs := make(chan error, 3)
	go func() { errs <- server.Serve(ln) }()
	if admin != nil {
		go func() { errs <- admin.Serve(adminLn) }()
	}
	if c != nil {
		go func() { errs <- c.Serve(ctx) }()
	}
	scheme := "http"
	if conf.TLSCert != "" {
		scheme = "https"
	}
	log.Printf("namedICMP %s panel %s://%s (panel-only=%v)", version, scheme, conf.PanelListen, *panelOnly)
	select {
	case <-ctx.Done():
	case err = <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
	}
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_ = server.Shutdown(shutdown)
	if admin != nil {
		_ = admin.Shutdown(shutdown)
	}
	if c != nil {
		c.Stop()
	}
	<-maintenanceDone
	return err
}

