package core

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"namedicmp/internal/store"
	"namedicmp/internal/wire"
)

// Exercise the core through the actual encrypted protobuf envelope without
// requiring a raw socket. Known external AEAD vectors are tested in wire.
type envelopeTransport struct {
	*mockTransport
	server, client *wire.Security
}

func (t *envelopeTransport) SetCredentials(c []wire.Credential) error {
	return t.server.SetCredentials(c)
}
func (t *envelopeTransport) Write(p Packet, m wire.Message) error {
	b, err := t.server.Encode(m)
	if err != nil {
		return err
	}
	decoded, _, err := t.client.Decode("server", p.EchoID, b)
	if err != nil {
		return err
	}
	return t.mockTransport.Write(p, decoded)
}

func TestEncryptedTCPUDPAndDefaultOff(t *testing.T) {
	for _, mode := range []string{"none", "aes128", "aes256", "chacha20"} {
		t.Run(mode, func(t *testing.T) {
			base, _, old := regressionCore(t)
			u, err := base.Store.PutUser(store.User{ID: old.ID, Name: old.Name, Enabled: true, Encryption: mode})
			if err != nil {
				t.Fatal(err)
			}
			tr := &envelopeTransport{mockTransport: &mockTransport{out: make(chan response, 4096), done: make(chan struct{})}, server: wire.NewSecurity(), client: wire.NewSecurity()}
			credential := wire.Credential{Key: u.Key, Mode: mode, Secret: u.EncryptionKey}
			if err := tr.client.SetCredentials([]wire.Credential{credential}); err != nil {
				t.Fatal(err)
			}
			c, err := New(base.Store, base.Config, tr)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Stop()
			send := func(m wire.Message) {
				t.Helper()
				b, err := tr.client.Encode(m)
				if err != nil {
					t.Fatal(err)
				}
				decoded, encryption, err := tr.server.Decode("192.0.2.1", 7, b)
				if err != nil || encryption != mode {
					t.Fatalf("incoming envelope: %s %v", encryption, err)
				}
				c.Handle(context.Background(), Packet{Source: "192.0.2.1", EchoID: 7, Message: decoded, Encryption: encryption})
			}
			udp := udpEcho(t)
			ut := udp.LocalAddr().String()
			datagram := bytes.Repeat([]byte("DNS/QUIC"), 512)
			m := wire.Message{Connection: "shared", Key: u.Key, ReplyType: 0, Target: ut, Payload: datagram}
			send(m)
			awaitUDP(t, tr.mockTransport, ut, datagram)
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			go func() {
				conn, err := ln.Accept()
				if err == nil {
					defer conn.Close()
					io.Copy(conn, conn)
				}
			}()
			m.Target, m.TCP, m.Window = ln.Addr().String(), true, 32
			m.Payload = wire.EncodeFrame(wire.Frame{DataKind: 1})
			send(m)
			receiver := NewReliable(32, 100*time.Millisecond)
			deadline := time.NewTimer(3 * time.Second)
			defer deadline.Stop()
			dataSent := false
			for {
				select {
				case <-deadline.C:
					t.Fatal("encrypted TCP transfer stalled")
				case answer := <-tr.out:
					if answer.m.Kind == 2 {
						t.Fatal("premature close")
					}
					if !answer.m.TCP {
						t.Fatal("TCP response missing protocol marker")
					}
					f, err := wire.DecodeFrame(answer.m.Payload)
					if err != nil {
						t.Fatal(err)
					}
					controls, delivery := receiver.Input(f, time.Now())
					for _, control := range controls {
						m.Payload = wire.EncodeFrame(control)
						send(m)
					}
					for _, v := range delivery {
						if v.DataKind == 2 && !dataSent {
							m.Payload = wire.EncodeFrame(wire.Frame{ID: 1, Data: []byte("encrypted TCP")})
							send(m)
							dataSent = true
						}
						if v.DataKind == 0 {
							if string(v.Data) != "encrypted TCP" {
								t.Fatal("TCP payload corruption")
							}
							m.TCP, m.Target, m.Payload = false, ut, []byte("UDP stays open")
							send(m)
							awaitUDP(t, tr.mockTransport, ut, m.Payload)
							got, _ := c.Store.GetUser(u.ID)
							expected := int64(len(datagram) + len("encrypted TCP") + len(m.Payload))
							if got.Upload != expected || got.Download != expected {
								t.Fatal("encryption envelope charged as application bytes")
							}
							return
						}
					}
				}
			}
		})
	}
}
