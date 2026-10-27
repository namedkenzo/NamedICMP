package core

import (
	"errors"
	"fmt"
	"log"
	"namedicmp/internal/wire"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type transportResult struct {
	packet Packet
	err    error
}
type DualTransport struct {
	sockets        map[int]*RawTransport
	in             chan transportResult
	done           chan struct{}
	once           sync.Once
	workers        sync.WaitGroup
	packet         *packetReader
	mode           string
	readerWarning  string
	receiveRetries atomic.Uint64
}

// Both families share authentication, accounting and the bounded paced writer.
// Failure to bind one family is visible without taking the working family down.
func ListenDual(v4, v6, mode string) (*DualTransport, []string, error) {
	if mode == "" {
		mode = "raw_ip"
	}
	if mode != "raw_ip" && mode != "raw_packet" {
		return nil, nil, errors.New("invalid reader mode")
	}
	t := &DualTransport{sockets: map[int]*RawTransport{}, in: make(chan transportResult, 128), done: make(chan struct{}), mode: mode}
	var warnings []string
	for family, address := range map[int]string{4: v4, 6: v6} {
		if address == "" {
			continue
		}
		socket, err := listenRawFamily(address, family)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("ICMPv%d unavailable: %v", family, err))
			continue
		}
		t.sockets[family] = socket
	}
	if len(t.sockets) == 0 {
		return nil, warnings, errors.New("no ICMP family could be opened")
	}
	if mode == "raw_packet" {
		reader, err := openPacketReader(t.sockets)
		if err != nil {
			t.mode = "raw_ip"
			t.readerWarning = fmt.Sprintf("AF_PACKET unavailable; raw IP remains active. Check CAP_NET_RAW and the service address-family profile: %v", err)
			warnings = append(warnings, t.readerWarning)
		} else {
			t.packet = reader
			t.startReader("raw packet capture", reader.Read)
			return t, warnings, nil
		}
	}
	for family, socket := range t.sockets {
		t.startReader(fmt.Sprintf("ICMPv%d", family), socket.Read)
	}
	return t, warnings, nil
}
func (t *DualTransport) startReader(name string, read func() (Packet, error)) {
	t.workers.Add(1)
	go func() {
		defer t.workers.Done()
		var retryDelay time.Duration
		var lastRetryNotice time.Time
		for {
			p, err := read()
			if err != nil && transientReceiveError(err) {
				t.receiveRetries.Add(1)
				if receiveErrorNotice(&lastRetryNotice) {
					log.Printf("temporary %s receive failure; retrying with bounded backoff: %v", name, err)
				}
				if !waitReceiveRetry(t.done, &retryDelay) {
					return
				}
				continue
			}
			retryDelay = 0
			if err != nil {
				err = fmt.Errorf("%s receive failed: %w", name, err)
			}
			select {
			case t.in <- transportResult{p, err}:
			case <-t.done:
				return
			}
			if err != nil {
				return
			}
		}
	}()
}
func (t *DualTransport) Read() (Packet, error) {
	select {
	case result := <-t.in:
		return result.packet, result.err
	case <-t.done:
		return Packet{}, net.ErrClosed
	}
}
func (t *DualTransport) ReceiveRetries() uint64 { return t.receiveRetries.Load() }
func (t *DualTransport) Write(p Packet, m wire.Message) error {
	family := p.Family
	if family == 0 {
		family = 4
	}
	if s := t.sockets[family]; s != nil {
		return s.Write(p, m)
	}
	return errors.New("ICMP family unavailable")
}
func (t *DualTransport) SetCredentials(credentials []wire.Credential) error {
	for _, s := range t.sockets {
		if err := s.SetCredentials(credentials); err != nil {
			return err
		}
	}
	return nil
}
func (t *DualTransport) Families() []int {
	var out []int
	for _, family := range []int{4, 6} {
		if t.sockets[family] != nil {
			out = append(out, family)
		}
	}
	return out
}
func (t *DualTransport) Close() error {
	t.once.Do(func() {
		close(t.done)
		if t.packet != nil {
			_ = t.packet.Close()
		}
		for _, s := range t.sockets {
			_ = s.Close()
		}
		t.workers.Wait()
	})
	return nil
}
func (c *Core) Families() []int {
	if t, ok := c.transport.(interface{ Families() []int }); ok {
		return t.Families()
	}
	return []int{4}
}
func (c *Core) ReaderMode() string {
	if t, ok := c.transport.(*DualTransport); ok {
		return t.mode
	}
	return "raw_ip"
}
func (c *Core) ReaderWarning() string {
	if t, ok := c.transport.(*DualTransport); ok {
		return t.readerWarning
	}
	return ""
}
