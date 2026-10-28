// Package outbound contains independently implemented SOCKS5 and CONNECT clients.
package outbound

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"syscall"
	"time"

	"namedicmp/internal/store"
)

var ErrResolve = errors.New("target DNS lookup failed")
var ErrPolicy = errors.New("target denied by outbound policy")
var ErrUDPUnsupported = errors.New("HTTP CONNECT does not support UDP; select SOCKS5")

func SplitTarget(target string) (string, int, error) {
	h, p, err := net.SplitHostPort(target)
	if err != nil || h == "" || len(h) > 253 || strings.ContainsAny(h, "\r\n /\\%") {
		return "", 0, errors.New("invalid target")
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 {
		return "", 0, errors.New("invalid target port")
	}
	return h, n, nil
}

var specialRanges = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("198.18.0.0/15")}

func safeIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range specialRanges {
		if cidr.Contains(a) {
			return false
		}
	}
	return true
}
func Resolve(ctx context.Context, target string, allowPrivate bool) (string, []net.IP, error) {
	h, p, err := SplitTarget(target)
	if err != nil {
		return "", nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", h)
	if err != nil || len(ips) == 0 {
		if err==nil {err=errors.New("no target addresses")}
		return "", nil, fmt.Errorf("%w: %w",ErrResolve,err)
	}
	for _, ip := range ips {
		if !allowPrivate && !safeIP(ip) {
			return "", nil, ErrPolicy
		}
	}
	// Prefer IPv4 for UDP destinations on the IPv4-only VPS installations this
	// project targets. Valid IPv6-only destinations remain available.
	for i, ip := range ips {
		if ip.To4() != nil {
			ips[0], ips[i] = ips[i], ips[0]
			break
		}
	}
	return net.JoinHostPort(ips[0].String(), strconv.Itoa(p)), ips, nil
}
func proxyConnect(ctx context.Context, o store.Outbound) (net.Conn, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	return d.DialContext(ctx, "tcp", net.JoinHostPort(o.Host, strconv.Itoa(o.Port)))
}
func writeAll(c net.Conn, b []byte) error {
	for len(b) > 0 {
		n, err := c.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrUnexpectedEOF
		}
		b = b[n:]
	}
	return nil
}
func socksAuth(c net.Conn, o store.Outbound) error {
	method := byte(0)
	if o.Username != "" {
		method = 2
	}
	if err := writeAll(c, []byte{5, 1, method}); err != nil {
		return err
	}
	var b [2]byte
	if _, err := io.ReadFull(c, b[:]); err != nil {
		return err
	}
	if b[0] != 5 || b[1] != method {
		return errors.New("SOCKS authentication method rejected")
	}
	if method == 2 {
		if err := writeAll(c, append(append([]byte{1, byte(len(o.Username))}, []byte(o.Username)...), append([]byte{byte(len(o.Password))}, []byte(o.Password)...)...)); err != nil {
			return err
		}
		if _, err := io.ReadFull(c, b[:]); err != nil {
			return err
		}
		if b[0] != 1 || b[1] != 0 {
			return errors.New("SOCKS authentication rejected")
		}
	}
	return nil
}
func Address(target string) ([]byte, error) {
	h, p, err := SplitTarget(target)
	if err != nil {
		return nil, err
	}
	var b []byte
	ip := net.ParseIP(h)
	if ip4 := ip.To4(); ip4 != nil {
		b = append([]byte{1}, ip4...)
	} else if ip != nil {
		b = append([]byte{4}, ip.To16()...)
	} else {
		b = append([]byte{3, byte(len(h))}, []byte(h)...)
	}
	return binary.BigEndian.AppendUint16(b, uint16(p)), nil
}
func readAddress(r io.Reader, kind byte) (string, error) {
	var b []byte
	switch kind {
	case 1:
		b = make([]byte, 4)
	case 4:
		b = make([]byte, 16)
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(r, n[:]); err != nil {
			return "", err
		}
		if n[0] == 0 {
			return "", errors.New("empty SOCKS host")
		}
		b = make([]byte, int(n[0]))
	default:
		return "", errors.New("invalid SOCKS address")
	}
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	var port [2]byte
	if _, err := io.ReadFull(r, port[:]); err != nil {
		return "", err
	}
	host := string(b)
	if kind != 3 {
		host = net.IP(b).String()
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port[:])))), nil
}
func socksRequest(c net.Conn, command byte, target string) (string, error) {
	addr, err := Address(target)
	if command == 3 {
		addr = []byte{1, 0, 0, 0, 0, 0, 0}
		err = nil
	}
	if err != nil {
		return "", err
	}
	if err = writeAll(c, append([]byte{5, command, 0}, addr...)); err != nil {
		return "", err
	}
	var b [4]byte
	if _, err = io.ReadFull(c, b[:]); err != nil {
		return "", err
	}
	if b[0] != 5 || b[1] != 0 || b[2] != 0 {
		return "", errors.New("SOCKS request rejected")
	}
	return readAddress(c, b[3])
}

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) CloseWrite() error {
	if x, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return x.CloseWrite()
	}
	return errors.New("proxy cannot half-close")
}

func (c *bufferedConn) Read(b []byte) (int, error) { return c.r.Read(b) }
func DialTCP(ctx context.Context, o store.Outbound, target string, allowPrivate bool) (net.Conn, error) {
	numeric, ips, err := Resolve(ctx, target, allowPrivate)
	if err != nil {
		return nil, err
	}
	if o.Kind == "direct" {
		_, port, _ := SplitTarget(target)
		var last error
		for _, ip := range ips {
			timeout := 10 * time.Second
			if len(ips) > 1 {
				timeout = 3 * time.Second
			}
			d := net.Dialer{Timeout: timeout}
			conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
			if err == nil {
				return conn, nil
			}
			last = err
			if ctx.Err() != nil {
				break
			}
		}
		return nil, last
	}
	c, err := proxyConnect(ctx, o)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			c.Close()
		}
	}()
	err = proxyHandshake(ctx, c, func(context.Context) error {
	switch o.Kind {
	case "socks5":
		if err = socksAuth(c, o); err != nil {
			return err
		}
		if _, err = socksRequest(c, 1, numeric); err != nil {
			return err
		}
	case "http":
		request := "CONNECT " + numeric + " HTTP/1.1\r\nHost: " + numeric + "\r\n"
		if o.Username != "" {
			request += "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(o.Username+":"+o.Password)) + "\r\n"
		}
		request += "\r\n"
		if err = writeAll(c, []byte(request)); err != nil {
			return err
		}
		r := bufio.NewReaderSize(c, 8192)
		line, e := limitedLine(r)
		if e != nil {
			return e
		}
		parts := strings.Fields(line)
		status := 0
		if len(parts) >= 2 && len(parts[1]) == 3 {
			status, _ = strconv.Atoi(parts[1])
		}
		if len(parts) < 2 || (parts[0] != "HTTP/1.1" && parts[0] != "HTTP/1.0") || status < 200 || status >= 300 {
			return errors.New("HTTP CONNECT rejected")
		}
		size := len(line)
		for {
			line, e = limitedLine(r)
			if e != nil {
				return e
			}
			size += len(line)
			if size > 16384 {
				return errors.New("proxy headers too large")
			}
			if line == "\r\n" || line == "\n" {
				break
			}
		}
		c = &bufferedConn{Conn: c, r: r}
	default:
		return errors.New("unsupported outbound")
	}
		return nil
	})
	if err != nil {
		return nil, err
	}
	ok = true
	return c, nil
}

type UDP struct {
	conn    *net.UDPConn
	control net.Conn
	target  string
	relay   bool
	valid   []net.IP
	port    int
	header  []byte
}

var ErrDropDatagram = errors.New("invalid or truncated UDP datagram")

func DialUDP(ctx context.Context, o store.Outbound, target string, allowPrivate bool) (*UDP, error) {
	numeric, ips, err := Resolve(ctx, target, allowPrivate)
	if err != nil {
		return nil, err
	}
	_, port, _ := SplitTarget(target)
	if o.Kind == "http" {
		return nil, ErrUDPUnsupported
	}
	if o.Kind == "direct" {
		addr, err := net.ResolveUDPAddr("udp", numeric)
		if err != nil {
			return nil, err
		}
		c, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return nil, err
		}
		return &UDP{conn: c, valid: ips, port: port, target: target}, nil
	}
	if o.Kind != "socks5" {
		return nil, errors.New("unsupported UDP outbound")
	}
	c, err := proxyConnect(ctx, o)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			c.Close()
		}
	}()
	var uc *net.UDPConn
	err = proxyHandshake(ctx, c, func(handshakeCtx context.Context) error {
		if err := socksAuth(c, o); err != nil {
			return err
		}
		relay, err := socksRequest(c, 3, "0.0.0.0:1")
		if err != nil {
			return err
		}
		host, port, err := net.SplitHostPort(relay)
		if err != nil {
			return err
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
			host, _, _ = net.SplitHostPort(c.RemoteAddr().String())
			relay = net.JoinHostPort(host, port)
		}
		conn, err := (&net.Dialer{}).DialContext(handshakeCtx, "udp", relay)
		if err != nil {
			return err
		}
		uc = conn.(*net.UDPConn)
		return nil
	})
	if err != nil {
		if uc != nil {
			uc.Close()
		}
		return nil, err
	}
	header, _ := Address(numeric)
	u := &UDP{conn: uc, control: c, relay: true, target: numeric, valid: ips, port: port, header: append([]byte{0, 0, 0}, header...)}
	ok = true
	return u, nil
}
func (u *UDP) Write(b []byte) error {
	if u.relay {
		packet := make([]byte, len(u.header)+len(b))
		copy(packet, u.header)
		copy(packet[len(u.header):], b)
		b = packet
	}
	_, err := u.conn.Write(b)
	return err
}
func (u *UDP) Read(b []byte) ([]byte, error) {
	n, _, flags, _, err := u.conn.ReadMsgUDP(b, nil)
	if err != nil {
		return nil, err
	}
	if flags&syscall.MSG_TRUNC != 0 {
		return nil, ErrDropDatagram
	}
	b = b[:n]
	if !u.relay {
		return b, nil
	}
	if len(b) < 4 || b[0] != 0 || b[1] != 0 || b[2] != 0 {
		return nil, ErrDropDatagram
	}
	r := bytes.NewReader(b[4:])
	target, err := readAddress(r, b[3])
	if err != nil {
		return nil, ErrDropDatagram
	}
	host, p, err := SplitTarget(target)
	if err != nil || p != u.port {
		return nil, ErrDropDatagram
	}
	ip := net.ParseIP(host)
	found := false
	for _, valid := range u.valid {
		if ip.Equal(valid) {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrDropDatagram
	}
	offset := len(b) - r.Len()
	return b[offset:], nil
}
func (u *UDP) SetDeadline(t time.Time) error { return u.conn.SetDeadline(t) }
func (u *UDP) Close() error {
	if u.control != nil {
		u.control.Close()
	}
	return u.conn.Close()
}
func (u *UDP) WatchControl(done func()) {
	if u.control == nil {
		return
	}
	go func() { var b [1]byte; _, _ = u.control.Read(b[:]); done() }()
}

