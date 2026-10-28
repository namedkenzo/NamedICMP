package outbound

import (
	"context"
	"net"
	"time"
)

// DialContext's context stops governing a socket after TCP connect. Proxy
// negotiation (and relay DNS) must separately obey the caller's cancellation.
func proxyHandshake(parent context.Context, c net.Conn, exchange func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, _ := ctx.Deadline()
	if err := c.SetDeadline(deadline); err != nil {
		return err
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		c.Close()
		close(done)
	})
	defer stop()
	err := exchange(ctx)
	if !stop() {
		<-done
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	// Detach cancellation before handing a live connection to its owner.
	return c.SetDeadline(time.Time{})
}
