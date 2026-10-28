package core

import "sync/atomic"

// Each rejected opening request has exactly one reason. These are packet/event
// counters, not a count of unique rejected connections or measured path loss.
type capacityReasons struct {
	sessionGlobalDrops   atomic.Uint64
	sessionTCPUserDrops  atomic.Uint64
	sessionUDPUserDrops  atomic.Uint64
	sessionStoppingDrops atomic.Uint64
}

type queueReasons struct {
	tcpQueueRetries     atomic.Uint64
	controlQueueDropped atomic.Uint64
	udpQueueEvicted     atomic.Uint64
	udpQueueRejected    atomic.Uint64
}

// Called with c.mu held, only after validating a genuine new flow. Existing
// sessions bypass admission so a full table cannot evict a working stream.
func (c *Core) rejectSessionLocked(user string, tcp bool) bool {
	udpLimit := c.Config.MaxUDPPerUser
	if udpLimit == 0 {
		udpLimit = 128
	}
	switch {
	case c.stopping:
		c.sessionStoppingDrops.Add(1)
	case len(c.sessions) >= c.Config.MaxSessions:
		c.sessionGlobalDrops.Add(1)
	case tcp && c.tcpPerUser[user] >= c.Config.MaxPerUser:
		c.sessionTCPUserDrops.Add(1)
	case !tcp && c.udpPerUser[user] >= udpLimit:
		c.sessionUDPUserDrops.Add(1)
	default:
		return false
	}
	c.dropped.Add(1)
	c.sessionLimitDrops.Add(1)
	return true
}
