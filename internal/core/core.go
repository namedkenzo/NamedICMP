package core

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	"namedicmp/internal/outbound"
	"namedicmp/internal/store"
	"namedicmp/internal/wire"
)

type Packet struct {
	Source          string
	EchoID, EchoSeq uint16
	Message         wire.Message
	Encryption      string
}

const MaxUDPDatagram = 8192

type sessionKey struct {
	User, Source, Connection, Target string
	TCP                              bool
}
type Transport interface {
	Read() (Packet, error)
	Write(Packet, wire.Message) error
	Close() error
}
type RawTransport struct {
	c        *net.IPConn
	buffer   []byte
	security *wire.Security
}

func ListenRaw(address string) (*RawTransport, error) {
	addr, err := net.ResolveIPAddr("ip4", address)
	if err != nil {
		return nil, err
	}
	c, err := net.ListenIP("ip4:icmp", addr)
	if err != nil {
		return nil, fmt.Errorf("ICMP socket requires CAP_NET_RAW: %w", err)
	}
	c.SetReadBuffer(4 << 20)
	c.SetWriteBuffer(4 << 20)
	return &RawTransport{c: c, buffer: make([]byte, 65535), security: wire.NewSecurity()}, nil
}
func (r *RawTransport) Read() (Packet, error) {
	b := r.buffer
	for {
		n, addr, err := r.c.ReadFromIP(b)
		if err != nil {
			return Packet{}, err
		}
		_, id, seq, p, err := wire.DecodeEcho(b[:n])
		if err != nil {
			continue
		}
		m, mode, err := r.security.Decode(addr.IP.String(), id, p)
		if err != nil || m.ReplyType != 0 && m.ReplyType != 8 {
			continue
		}
		if mode == "none" {
			m.Payload = append([]byte(nil), m.Payload...)
		} // encrypted plaintext already owns its buffer
		return Packet{Source: addr.IP.String(), EchoID: id, EchoSeq: seq, Message: m, Encryption: mode}, nil
	}
}
func (r *RawTransport) Write(p Packet, m wire.Message) error {
	payload, err := r.security.Encode(m)
	if err != nil {
		return err
	}
	b := wire.Echo(byte(p.Message.ReplyType), p.EchoID, p.EchoSeq, payload)
	// This socket has one writer, so this deadline cannot affect another send.
	if err:=r.c.SetWriteDeadline(time.Now().Add(200*time.Millisecond));err!=nil{return err}
	_, err = r.c.WriteToIP(b, &net.IPAddr{IP: net.ParseIP(p.Source)})
	return err
}
func (r *RawTransport) Close() error                             { return r.c.Close() }
func (r *RawTransport) SetCredentials(c []wire.Credential) error { return r.security.SetCredentials(c) }

type Core struct {
	Store                                                  *store.Store
	Config                                                 store.Config
	transport                                              Transport
	mu                                                     sync.Mutex
	sessions                                               map[sessionKey]*session
	perUser                                                map[string]int
	udpPerUser                                             map[string]int
	tcpPerUser                                             map[string]int
	wg                                                     sync.WaitGroup
	invalid, dropped, received, sent                       atomic.Uint64
	udpIn, udpOut, tcpIn, tcpOut, dialErrors, oversizedUDP atomic.Uint64
	decode                                                 *zstd.Decoder
	stopping                                               bool
	controlOut, dataOut chan responsePacket
	writeDone, stopWrite chan struct{}
	writeQueueDrops, sessionQueueDrops, sessionLimitDrops, socketWriteErrors atomic.Uint64
	outboundDNS, outboundPolicy, outboundTimeout, outboundNetwork, outboundOther atomic.Uint64
	stopOnce                                               sync.Once
	credentialsMu                                          sync.Mutex
	pingMu                                                 sync.Mutex
	pingBuckets                                            map[string]*bucket
}
type bucket struct {
	tokens float64
	at     time.Time
}

func New(s *store.Store, c store.Config, t Transport) (*Core, error) {
	d, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(1<<20), zstd.WithDecodeAllCapLimit(true))
	if err != nil {
		return nil, err
	}
	core := &Core{Store: s, Config: c, transport: t, sessions: map[sessionKey]*session{}, perUser: map[string]int{}, udpPerUser: map[string]int{}, tcpPerUser: map[string]int{}, decode: d, pingBuckets: map[string]*bucket{}, controlOut:make(chan responsePacket,256),dataOut:make(chan responsePacket,2048),writeDone:make(chan struct{}),stopWrite:make(chan struct{})}
	if err := core.RefreshCredentials(); err != nil {
		d.Close()
		return nil, err
	}
	go core.writeLoop()
	return core, nil
}
func (c *Core) RefreshCredentials() error {
	c.credentialsMu.Lock()
	defer c.credentialsMu.Unlock()
	t, ok := c.transport.(interface{ SetCredentials([]wire.Credential) error })
	if !ok {
		return nil
	}
	users := c.Store.Users()
	credentials := make([]wire.Credential, 0, len(users))
	for _, u := range users {
		credentials = append(credentials, wire.Credential{Key: u.Key, Mode: u.Encryption, Secret: u.EncryptionKey})
	}
	return t.SetCredentials(credentials)
}
func (c *Core) allowPing(id string) bool {
	c.pingMu.Lock()
	defer c.pingMu.Unlock()
	now := time.Now()
	b := c.pingBuckets[id]
	if b == nil {
		if len(c.pingBuckets) >= 10000 {
			return false
		}
		b = &bucket{tokens: 20, at: now}
		c.pingBuckets[id] = b
	}
	b.tokens = min(20, b.tokens+now.Sub(b.at).Seconds()*5)
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
func identity(p Packet, user string) sessionKey {
	k := sessionKey{User: user, Source: p.Source, Connection: p.Message.Connection, TCP: p.Message.TCP}
	if !k.TCP {
		k.Target = p.Message.Target
	}
	return k
}
func (c *Core) Serve(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			c.transport.Close()
		case <-done:
		}
	}()
	defer close(done)
	for {
		p, err := c.transport.Read()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		c.Handle(ctx, p)
	}
}
func (c *Core) Handle(ctx context.Context, p Packet) {
	c.received.Add(1)
	m := p.Message
	u, ok := c.Store.Lookup(m.Key)
	mode, _ := wire.NormalizeEncryption(p.Encryption)
	if !ok || mode != u.Encryption || !u.Allowed(time.Now()) || m.ReplyType != 0 && m.ReplyType != 8 {
		c.invalid.Add(1)
		return
	}
	if m.Kind == 1 {
		if len(m.Payload) <= 64 && c.allowPing(u.ID) {
			c.reply(p, wire.Message{Kind: 1, Key: u.Key, ReplyType: -1, Payload: m.Payload})
		}
		return
	}
	if m.Kind == 2 {
		c.mu.Lock()
		// Legacy KICK has no protocol/target. Its scope is the authenticated ID.
		for k, s := range c.sessions {
			if k.User == u.ID && k.Source == p.Source && k.Connection == m.Connection && (m.Target == "" || k == identity(p, u.ID)) {
				s.cancel()
			}
		}
		c.mu.Unlock()
		return
	}
	if m.Connection == "" || m.Target == "" || len(m.Payload) > wire.MaxMessage {
		return
	}
	if !m.TCP && len(m.Payload) > MaxUDPDatagram {
		c.oversizedUDP.Add(1)
		c.dropped.Add(1)
		return
	}
	if m.TCP {
		c.tcpIn.Add(1)
	} else {
		c.udpIn.Add(1)
	}
	key := identity(p, u.ID)
	c.mu.Lock()
	s := c.sessions[key]
	if s == nil {
		limit := c.Config.MaxUDPPerUser
		if limit == 0 {
			limit = 128
		}
		userFull := (!m.TCP && c.udpPerUser[u.ID] >= limit) || (m.TCP && c.tcpPerUser[u.ID] >= c.Config.MaxPerUser)
		if c.stopping || len(c.sessions) >= c.Config.MaxSessions || userFull {
			c.mu.Unlock()
			c.dropped.Add(1)
			c.sessionLimitDrops.Add(1)
			if m.TCP {
				c.kick(p)
			}
			return
		}
		if _, _, err := outbound.SplitTarget(m.Target); err != nil {
			c.mu.Unlock()
			c.invalid.Add(1)
			return
		}
		if m.TCP {
			f, err := wire.DecodeFrame(m.Payload)
			if err != nil || f.Kind != 0 || f.ID != 0 || f.DataKind != 1 {
				c.mu.Unlock()
				return
			}
		}
		s = newSession(ctx, c, key, u, p)
		c.sessions[key] = s
		c.perUser[u.ID]++
		if m.TCP {
			c.tcpPerUser[u.ID]++
		} else {
			c.udpPerUser[u.ID]++
		}
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			s.run()
			c.mu.Lock()
			delete(c.sessions, key)
			c.perUser[u.ID]--
			if m.TCP {
				c.tcpPerUser[u.ID]--
			} else {
				c.udpPerUser[u.ID]--
			}
			c.mu.Unlock()
		}()
	}
	c.mu.Unlock()
	select {
	case s.in <- p:
	default:
		c.dropped.Add(1)
		c.sessionQueueDrops.Add(1)
	}
}
func (c *Core) reply(p Packet, m wire.Message) {
	m.TCP = p.Message.TCP
	queue:=c.dataOut
	if m.Kind!=0 {queue=c.controlOut} else if m.TCP {
		if f,err:=wire.DecodeFrame(m.Payload);err==nil && (f.Kind!=0 || f.DataKind!=0) {queue=c.controlOut}
	}
	select {case <-c.stopWrite:return;default:}
	select {case queue<-responsePacket{p,m}:default:c.dropped.Add(1);c.writeQueueDrops.Add(1)}
}
func (c *Core) write(p Packet,m wire.Message) {
	if err := c.transport.Write(p, m); err == nil {
		c.sent.Add(1)
		if m.Kind == 0 {
			if p.Message.TCP {
				c.tcpOut.Add(1)
			} else {
				c.udpOut.Add(1)
			}
		}
	} else {
		c.dropped.Add(1)
		c.socketWriteErrors.Add(1)
	}
}
func (c *Core) kick(p Packet) {
	c.reply(p, wire.Message{Connection: p.Message.Connection, Kind: 2, Key: p.Message.Key, ReplyType: -1})
}
func (c *Core) Revoke(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.sessions {
		if s.user.ID == id {
			s.cancel()
		}
	}
}
func (c *Core) Counts() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]int{}
	for id, n := range c.perUser {
		out[id] = n
	}
	return out
}
func (c *Core) Metrics() map[string]any {
	c.mu.Lock()
	n := len(c.sessions)
	c.mu.Unlock()
	return map[string]any{"sessions": n, "tcp_packets_in": c.tcpIn.Load(), "tcp_packets_out": c.tcpOut.Load(), "udp_packets_in": c.udpIn.Load(), "udp_packets_out": c.udpOut.Load(), "outbound_errors": c.dialErrors.Load(), "outbound_dns_errors":c.outboundDNS.Load(),"outbound_policy_errors":c.outboundPolicy.Load(),"outbound_timeouts":c.outboundTimeout.Load(),"outbound_network_errors":c.outboundNetwork.Load(),"outbound_other_errors":c.outboundOther.Load(),"write_queue_dropped":c.writeQueueDrops.Load(),"session_queue_dropped":c.sessionQueueDrops.Load(),"session_limit_rejected":c.sessionLimitDrops.Load(),"socket_write_errors":c.socketWriteErrors.Load(),"icmp_packets_per_second":c.Config.ICMPPacketsPerSecond,"icmp_burst_packets":c.Config.ICMPBurstPackets, "oversized_udp": c.oversizedUDP.Load(), "packets_in": c.received.Load(), "packets_out": c.sent.Load(), "dropped": c.dropped.Load(), "rejected": c.invalid.Load(), "state_healthy": c.Store.Fault() == nil}
}
func (c *Core) Stop() {
	c.stopOnce.Do(func() {
		c.mu.Lock()
		c.stopping = true
		for _, s := range c.sessions {
			s.cancel()
		}
		c.mu.Unlock()
		close(c.stopWrite)
		c.transport.Close()
		<-c.writeDone
		c.wg.Wait()
		c.decode.Close()
	})
}
func (c *Core) Maintain(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.mu.Lock()
			for _, s := range c.sessions {
				u, ok := c.Store.GetUser(s.user.ID)
				if !ok || !u.Allowed(time.Now()) || c.Store.Fault() != nil {
					s.cancel()
				}
			}
			c.mu.Unlock()
			if err := c.Store.Flush(); err != nil {
				log.Printf("usage storage failed; tunnel traffic denied: %v", err)
			}
			c.pingMu.Lock()
			now := time.Now()
			for id, b := range c.pingBuckets {
				if now.Sub(b.at) > time.Minute {
					delete(c.pingBuckets, id)
				}
			}
			c.pingMu.Unlock()
		}
	}
}

