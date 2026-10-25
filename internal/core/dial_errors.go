package core

import (
 "context"
 "errors"
 "net"
 "namedicmp/internal/outbound"
)

// Aggregate categories only: no destinations, user IDs or credentials in logs.
func(c *Core) recordDialError(err error) {
 c.dialErrors.Add(1)
 var timeout net.Error
 switch {
 case errors.Is(err,outbound.ErrResolve):c.outboundDNS.Add(1)
 case errors.Is(err,outbound.ErrPolicy)||errors.Is(err,outbound.ErrUDPUnsupported):c.outboundPolicy.Add(1)
 case errors.Is(err,context.DeadlineExceeded)||errors.As(err,&timeout)&&timeout.Timeout():c.outboundTimeout.Add(1)
 default:
  var network *net.OpError
  if errors.As(err,&network) {c.outboundNetwork.Add(1)}else{c.outboundOther.Add(1)}
 }
}
