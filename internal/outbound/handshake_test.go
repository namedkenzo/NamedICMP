package outbound

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// Exercise actual TCP sockets and public dial entry points, including an
// established TCP connection whose proxy negotiation has not completed.
func TestProxyHandshakeCancellation(t *testing.T) {
	for _, phase := range []string{"http_headers", "socks_auth", "socks_connect", "socks_udp"} {
		t.Run(phase, func(t *testing.T) {
			reached := make(chan net.Conn, 1)
			o := proxy(t, func(c net.Conn) {
				if phase == "http_headers" {
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
				} else {
					var greet [3]byte
					if _, err := io.ReadFull(c, greet[:]); err != nil {
						return
					}
					if phase != "socks_auth" {
						c.Write([]byte{5, 0})
						var req [4]byte
						if _, err := io.ReadFull(c, req[:]); err != nil {
							return
						}
						if _, err := readAddress(c, req[3]); err != nil {
							return
						}
					}
				}
				reached <- c
				io.Copy(io.Discard, c)
			})
			o.Kind = "socks5"
			if phase == "http_headers" {
				o.Kind = "http"
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				if phase == "socks_udp" {
					u, err := DialUDP(ctx, o, "127.0.0.1:53", true)
					if u != nil {
						u.Close()
					}
					result <- err
				} else {
					c, err := DialTCP(ctx, o, "127.0.0.1:443", true)
					if c != nil {
						c.Close()
					}
					result <- err
				}
			}()
			var peer net.Conn
			select {
			case peer = <-reached:
			case <-time.After(2 * time.Second):
				t.Fatal("proxy handshake not reached")
			}
			defer peer.Close()
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("wrong cancellation result: %v", err)
				}
			case <-time.After(time.Second):
				peer.Close()
				<-result
				t.Fatal("canceled proxy handshake retained its connection")
			}
		})
	}
}

func TestProxyDeadlineAndSuccessfulHandoff(t *testing.T) {
	for _, cancelDuring := range []bool{true, false} {
		t.Run(map[bool]string{true: "deadline", false: "handoff"}[cancelDuring], func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			timeout := 2 * time.Second
			if cancelDuring {
				timeout = 30 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := proxyHandshake(ctx, a, func(context.Context) error {
				if cancelDuring {
					var data [1]byte
					_, err := a.Read(data[:])
					return err
				}
				return nil
			})
			if cancelDuring {
				// The socket deadline and context timer can wake in either order;
				// both are bounded timeout failures.
				if err == nil {
					t.Fatal("expired handshake succeeded")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			cancel()
			a.SetDeadline(time.Now().Add(time.Second))
			go b.Write([]byte("x"))
			var data [1]byte
			if _, err := io.ReadFull(a, data[:]); err != nil {
				t.Fatalf("completed handshake still owned by context: %v", err)
			}
		})
	}
}

func TestHTTPConnectSuccessStatuses(t *testing.T) {
	for _, status := range []string{"200", "201", "204", "299", "300", "407", "2000", "2x0"} {
		t.Run(status, func(t *testing.T) {
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
				io.WriteString(c, "HTTP/1.1 "+status+" Result\r\n\r\nTUNNEL")
			})
			o.Kind = "http"
			c, err := DialTCP(context.Background(), o, "127.0.0.1:443", true)
			good := status == "200" || status == "201" || status == "204" || status == "299"
			if !good {
				if c != nil {
					c.Close()
				}
				if err == nil {
					t.Fatal("invalid CONNECT response accepted")
				}
				return
			}
			if err != nil {
				t.Fatalf("valid CONNECT 2xx rejected: %v", err)
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(time.Second))
			data := make([]byte, 6)
			if _, err := io.ReadFull(c, data); err != nil || string(data) != "TUNNEL" {
				t.Fatalf("tunnel bytes lost: %q %v", data, err)
			}
		})
	}
}
