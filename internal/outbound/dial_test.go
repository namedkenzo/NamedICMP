package outbound

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"namedicmp/internal/store"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func proxy(t *testing.T, serve func(net.Conn)) store.Outbound {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err == nil {
			defer c.Close()
			serve(c)
		}
	}()
	h, p, _ := net.SplitHostPort(ln.Addr().String())
	n, _ := strconv.Atoi(p)
	return store.Outbound{Host: h, Port: n}
}
func TestHTTPPreservesBufferedTunnelBytes(t *testing.T) {
	o := proxy(t, func(c net.Conn) {
		r := bufio.NewReader(c)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		io.WriteString(c, "HTTP/1.1 200 Connection Established\r\nX-Test: yes\r\n\r\nGREETING")
		io.Copy(c, c)
	})
	o.Kind = "http"
	c, err := DialTCP(context.Background(), o, "127.0.0.1:1234", true)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
	b := make([]byte, 8)
	if _, err = io.ReadFull(c, b); err != nil || string(b) != "GREETING" {
		t.Fatalf("prefetched data lost: %q %v", b, err)
	}
	c.Write([]byte("echo"))
	b = make([]byte, 4)
	io.ReadFull(c, b)
	if string(b) != "echo" {
		t.Fatal("proxy echo")
	}
}
func TestSOCKSAuthAndCONNECT(t *testing.T) {
	o := proxy(t, func(c net.Conn) {
		var greeting [3]byte
		io.ReadFull(c, greeting[:])
		if greeting != [3]byte{5, 1, 2} {
			return
		}
		c.Write([]byte{5, 2})
		var b [2]byte
		io.ReadFull(c, b[:])
		user := make([]byte, int(b[1]))
		io.ReadFull(c, user)
		var n [1]byte
		io.ReadFull(c, n[:])
		pass := make([]byte, int(n[0]))
		io.ReadFull(c, pass)
		if string(user) != "user" || string(pass) != "secret" {
			return
		}
		c.Write([]byte{1, 0})
		var req [4]byte
		io.ReadFull(c, req[:])
		if req[1] != 1 {
			return
		}
		_, err := readAddress(c, req[3])
		if err != nil {
			return
		}
		c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 1})
		io.Copy(c, c)
	})
	o.Kind = "socks5"
	o.Username = "user"
	o.Password = "secret"
	c, err := DialTCP(context.Background(), o, "127.0.0.1:1234", true)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
	c.Write([]byte("test"))
	b := make([]byte, 4)
	if _, err = io.ReadFull(c, b); err != nil || string(b) != "test" {
		t.Fatal("SOCKS payload")
	}
}
func TestSOCKSUDPAssociation(t *testing.T) {
	relay, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	go func() {
		b := make([]byte, 2000)
		n, peer, e := relay.ReadFromUDP(b)
		if e == nil {
			// Malformed and fragmented datagrams must not poison the association.
			relay.WriteToUDP([]byte{0, 0}, peer)
			fragment := append([]byte(nil), b[:n]...)
			fragment[2] = 1
			relay.WriteToUDP(fragment, peer)
			relay.WriteToUDP(b[:n], peer)
		}
	}()
	o := proxy(t, func(c net.Conn) {
		var greet [3]byte
		io.ReadFull(c, greet[:])
		c.Write([]byte{5, 0})
		var req [4]byte
		io.ReadFull(c, req[:])
		addr, _ := readAddress(c, req[3])
		if req[1] != 3 || addr != "0.0.0.0:0" {
			return
		}
		b := []byte{5, 0, 0, 1, 127, 0, 0, 1}
		b = binary.BigEndian.AppendUint16(b, uint16(relay.LocalAddr().(*net.UDPAddr).Port))
		c.Write(b)
		io.Copy(io.Discard, c)
	})
	o.Kind = "socks5"
	u, err := DialUDP(context.Background(), o, "127.0.0.1:5353", true)
	if err != nil {
		t.Fatal(err)
	}
	defer u.Close()
	u.SetDeadline(time.Now().Add(time.Second))
	if err = u.Write([]byte("DNS")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := u.Read(make([]byte, 2000)); !errors.Is(err, ErrDropDatagram) {
			t.Fatalf("bad relay datagram not dropped: %v", err)
		}
	}
	data, err := u.Read(make([]byte, 2000))
	if err != nil || string(data) != "DNS" {
		t.Fatalf("UDP proxy: %q %v", data, err)
	}
}

func TestUDPTruncationDropsOnlyOneDatagram(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go func() {
		b := make([]byte, 1)
		_, peer, err := server.ReadFromUDP(b)
		if err == nil {
			server.WriteToUDP(make([]byte, 4096), peer)
			server.WriteToUDP([]byte("intact"), peer)
		}
	}()
	u, err := DialUDP(context.Background(), store.Outbound{Kind: "direct"}, server.LocalAddr().String(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer u.Close()
	u.SetDeadline(time.Now().Add(time.Second))
	u.Write([]byte{1})
	if _, err := u.Read(make([]byte, 64)); !errors.Is(err, ErrDropDatagram) {
		t.Fatal("truncated payload delivered as valid UDP")
	}
	data, err := u.Read(make([]byte, 64))
	if err != nil || string(data) != "intact" {
		t.Fatalf("next datagram lost: %q %v", data, err)
	}
}
func TestPolicyAndNoSilentBypass(t *testing.T) {
	for _, target := range []string{"127.0.0.1:80", "10.1.2.3:443", "169.254.169.254:80", "[::1]:22"} {
		if _, _, err := Resolve(context.Background(), target, false); err == nil {
			t.Fatalf("private target accepted: %s", target)
		}
	}
	if _, err := DialUDP(context.Background(), store.Outbound{Kind: "http"}, "127.0.0.1:53", true); err == nil || !strings.Contains(err.Error(), "does not support UDP") {
		t.Fatal("HTTP UDP did not fail explicitly")
	}
}
