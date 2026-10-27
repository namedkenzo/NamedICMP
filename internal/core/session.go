package core

import (
	"context"
	"errors"
	"net"
	"time"

	"namedicmp/internal/outbound"
	"namedicmp/internal/store"
	"namedicmp/internal/wire"
)

type session struct {
	ctx     context.Context
	cancel  context.CancelFunc
	core    *Core
	key     sessionKey
	user    store.User
	peer    Packet
	in      chan Packet
	timeout time.Duration
}

func newSession(ctx context.Context, c *Core, key sessionKey, u store.User, p Packet) *session {
	x, cancel := context.WithCancel(ctx)
	timeout := time.Duration(p.Message.Timeout) * time.Second
	if timeout < 10*time.Second {
		timeout = 60 * time.Second
	}
	timeout = min(timeout, 10*time.Minute)
	return &session{ctx: x, cancel: cancel, core: c, key: key, user: u, peer: p, in: make(chan Packet, min(512, c.Config.Window*2+16)), timeout: timeout}
}
func (s *session) run() {
	defer s.cancel()
	// UDP has no CLOSE frame. A legacy KICK could also close a TCP flow whose
	// client ID is shared, so UDP expiry/error must never emit that control.
	if s.peer.Message.TCP {
		defer s.core.kick(s.peer)
	}
	o, ok := s.core.Store.GetOutbound(s.user.Outbound)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	if s.peer.Message.TCP {
		conn, err := outbound.DialTCP(ctx, o, s.peer.Message.Target, s.core.Config.AllowPrivate)
		if err != nil {
			s.core.recordDialError(err)
			return
		}
		defer conn.Close()
		s.tcp(conn)
	} else {
		conn, err := outbound.DialUDP(ctx, o, s.peer.Message.Target, s.core.Config.AllowPrivate)
		if err != nil {
			s.core.recordDialError(err)
			return
		}
		defer conn.Close()
		s.udp(conn)
	}
}
func (s *session) message(data []byte, target string) {
	s.core.reply(s.peer, wire.Message{Connection: s.peer.Message.Connection, Key: s.user.Key, ReplyType: -1, Payload: data, Target: target})
}
func (s *session) frame(f wire.Frame) { s.message(wire.EncodeFrame(f), "") }
func (s *session) updatePeer(p Packet) bool {
	if p.Message.Target != s.peer.Message.Target || p.Message.TCP != s.peer.Message.TCP {
		return false
	}
	s.peer.EchoID = p.EchoID
	s.peer.EchoSeq = p.EchoSeq
	s.peer.Message.ReplyType = p.Message.ReplyType
	return true
}

type readResult struct {
	data []byte
	err  error
}

func (s *session) udp(c *outbound.UDP) {
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-s.ctx.Done():
			c.Close()
		case <-stop:
		}
	}()
	c.WatchControl(s.cancel)
	results := make(chan readResult, 1)
	go func() {
		buf := make([]byte, MaxUDPDatagram+300)
		for {
			data, err := c.Read(buf)
			if errors.Is(err, outbound.ErrDropDatagram) {
				s.core.dropped.Add(1)
				continue
			}
			r := readResult{data: append([]byte(nil), data...), err: err}
			select {
			case results <- r:
			case <-s.ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	idle := time.NewTimer(s.timeout)
	defer idle.Stop()
	reset := func() {
		if !idle.Stop() {
			select {
			case <-idle.C:
			default:
			}
		}
		idle.Reset(s.timeout)
	}
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-idle.C:
			return
		case p := <-s.in:
			if !s.updatePeer(p) || len(p.Message.Payload) > MaxUDPDatagram {
				continue
			}
			if err := s.core.Store.Charge(s.user.ID, len(p.Message.Payload), true); err != nil {
				return
			}
			if err := c.Write(p.Message.Payload); err != nil {
				return
			}
			s.core.payloadUpload.Add(uint64(len(p.Message.Payload)))
			reset()
		case r := <-results:
			if r.err != nil {
				return
			}
			if len(r.data) > MaxUDPDatagram {
				s.core.oversizedUDP.Add(1)
				s.core.dropped.Add(1)
				continue
			}
			if err := s.core.Store.Charge(s.user.ID, len(r.data), false); err != nil {
				return
			}
			s.message(r.data, s.peer.Message.Target)
			s.core.payloadDownload.Add(uint64(len(r.data)))
			reset()
		}
	}
}
func (s *session) tcp(conn net.Conn) {
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-s.ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()
	window := int(s.peer.Message.Window)
	if window < 8 {
		window = 128
	}
	window = min(window, s.core.Config.Window)
	r := NewReliable(window, time.Duration(s.peer.Message.ResendMS)*time.Millisecond)
	// One reusable read buffer batches up to 32 frames; the receiver grants
	// byte credit only after it has copied the previous batch into frame storage.
	credit := make(chan int, 1)
	results := make(chan readResult, 1)
	go func() {
		buffer := make([]byte, 32*wire.PayloadSize)
		for {
			limit := 0
			select {
			case <-s.ctx.Done():
				return
			case limit = <-credit:
			}
			n, err := conn.Read(buffer[:limit])
			select {
			case results <- readResult{data: buffer[:n], err: err}:
			case <-s.ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	connected, eof, remoteClose, reading := false, false, false, false
	readingSlots := 0
	var pendingOutput []byte
	closeQueued := false
	lastActivity, lastHB := time.Now(), time.Now()
	closeAt := time.Time{}
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		now := time.Now()
		if !connected && now.Sub(lastActivity) > 10*time.Second {
			return
		}
		if now.Sub(lastActivity) > s.timeout {
			return
		}
		for len(pendingOutput) > 0 && r.Space() > 0 {
			size := min(wire.PayloadSize, len(pendingOutput))
			chunk := pendingOutput[:size]
			if err := s.core.Store.Charge(s.user.ID, size, false); err != nil {
				return
			}
			if err := r.Queue(0, chunk); err != nil {
				return
			}
			pendingOutput = pendingOutput[size:]
			s.core.payloadDownload.Add(uint64(size))
			lastActivity = now
		}
		if len(pendingOutput) == 0 {
			pendingOutput = nil
		}
		if connected && !eof && r.Space() > 0 && !reading && pendingOutput == nil {
			readingSlots = min(32, r.Space())
			credit <- readingSlots * wire.PayloadSize
			reading = true
		}
		if eof && r.Space() > 0 && !closeQueued && !reading && pendingOutput == nil {
			_ = r.Queue(3, nil)
			closeQueued = true
			closeAt = now
		}
		if closeQueued && (remoteClose && r.Empty() || now.Sub(closeAt) > 30*time.Second) {
			return
		}
		reservedSlot := 0
		if reading {
			reservedSlot = readingSlots
		}
		if connected && !eof && now.Sub(lastHB) >= time.Second && r.Space() > reservedSlot {
			_ = r.Queue(4, nil)
			lastHB = now
		}
		for _, f := range r.Due(now) {
			s.frame(f)
		}
		wake := min(r.Next(now), time.Second)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(max(wake, time.Millisecond))
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
		case p := <-s.in:
			if !s.updatePeer(p) {
				continue
			}
			f, err := wire.DecodeFrame(p.Message.Payload)
			if err != nil {
				return
			}
			if f.Kind == 0 && len(f.Data) > wire.PayloadSize {
				return
			}
			if f.Compressed {
				if f.DataKind != 0 {
					return
				}
				data, err := s.core.decode.DecodeAll(f.Data, make([]byte, 0, wire.PayloadSize))
				if err != nil || len(data) > wire.PayloadSize {
					return
				}
				f.Data = data
				f.Compressed = false
			}
			controls, deliver := r.Input(f, time.Now())
			for _, control := range controls {
				s.frame(control)
			}
			for _, v := range deliver {
				switch v.DataKind {
				case 0:
					if !connected || remoteClose {
						return
					}
					if err := s.core.Store.Charge(s.user.ID, len(v.Data), true); err != nil {
						return
					}
					conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if err := writeTCP(conn, v.Data); err != nil {
						return
					}
					conn.SetWriteDeadline(time.Time{})
					s.core.payloadUpload.Add(uint64(len(v.Data)))
					lastActivity = time.Now()
				case 1:
					if !connected {
						if err := r.Queue(2, nil); err != nil {
							return
						}
						connected = true
						lastActivity = time.Now()
					}
				case 2:
					return // server receives CONN, never a client CONNRSP
				case 3:
					remoteClose = true
					if tcp, ok := conn.(interface{ CloseWrite() error }); ok {
						_ = tcp.CloseWrite()
					} else {
						eof = true
					}
					if closeAt.IsZero() {
						closeAt = time.Now()
					}
				case 4: // protocol heartbeat is not application activity
				}
			}
		case result := <-results:
			reading = false
			readingSlots = 0
			if len(result.data) > 0 {
				pendingOutput = result.data
				lastActivity = time.Now()
			}
			if result.err != nil {
				eof = true
			}
		}
	}
}
func writeTCP(c net.Conn, b []byte) error {
	for len(b) > 0 {
		n, err := c.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("zero length write")
		}
		b = b[n:]
	}
	return nil
}
