package core

import (
	"errors"
	"syscall"
	"time"
)

// Raw receive sockets can report temporary kernel pressure or interruption.
// Do not treat arbitrary read errors, a closed socket, or a configuration error
// as transient: those must remain visible to the service supervisor.
func transientReceiveError(err error) bool {
	return errors.Is(err, syscall.EINTR) || errors.Is(err, syscall.EAGAIN) ||
		errors.Is(err, syscall.ENOBUFS) || errors.Is(err, syscall.ENOMEM)
}

func receiveErrorNotice(last *time.Time) bool {
	if !last.IsZero() && time.Since(*last) < 30*time.Second {
		return false
	}
	*last = time.Now()
	return true
}

// Capped delay prevents a persistently failing socket from spinning while a
// cancellation/transport close interrupts even the longest backoff immediately.
func waitReceiveRetry(done <-chan struct{}, delay *time.Duration) bool {
	if *delay <= 0 {
		*delay = 5 * time.Millisecond
	} else {
		*delay = min(500*time.Millisecond, *delay*2)
	}
	timer := time.NewTimer(*delay)
	defer timer.Stop()
	select {
	case <-done:
		return false
	case <-timer.C:
		return true
	}
}
