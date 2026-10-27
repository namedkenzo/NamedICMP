package core

import (
 "time"
 "namedicmp/internal/wire"
)

type responsePacket struct { peer Packet; message wire.Message; complete func(time.Time,bool) }

// One bounded writer prevents a busy data flow from blocking the receive loop.
// ACK/handshake/probe messages have a reserved queue; eight control packets are
// followed by a waiting data packet so neither class can starve the other.
func(c *Core) writeLoop() {
 defer close(c.writeDone)
 limit:=newPacketPacer(c.Config.ICMPPacketsPerSecond,c.Config.ICMPBurstPackets,time.Now())
 timer:=time.NewTimer(time.Hour);defer timer.Stop()
 controls,datagrams:=0,0
 for {
  var packet responsePacket
  found:=false
  if controls<8 {select{case packet=<-c.controlOut:found=true;controls++;default:}}
  if !found {packet,found=c.nextData(&datagrams);if found{controls=0}}
  if !found {select{case <-c.stopWrite:return;case packet=<-c.controlOut:controls++;case packet=<-c.udpOutQueue:controls=0;datagrams++;case packet=<-c.dataOut:controls=0;datagrams=0}}
  for {
   delay:=limit.take(time.Now())
   if delay<=0 {break}
   if !timer.Stop(){select{case <-timer.C:default:}}
   timer.Reset(delay)
   select{case <-c.stopWrite:return;case <-timer.C:}
  }
  select{case <-c.stopWrite:return;default:}
  sentAt:=time.Now()
  ok:=c.write(packet.peer,packet.message)
  if packet.complete!=nil {packet.complete(sentAt,ok)}
 }
}

// Reserve four UDP opportunities per waiting bulk TCP packet. Both remain
// paced by the same global limit, and control messages keep their reservation.
func(c *Core)nextData(datagrams *int)(responsePacket,bool){
 if *datagrams<4{select{case p:=<-c.udpOutQueue:*datagrams++;return p,true;default:}}
 select{case p:=<-c.dataOut:*datagrams=0;return p,true;default:}
 select{case p:=<-c.udpOutQueue:*datagrams=4;return p,true;default:}
 return responsePacket{},false
}

type packetPacer struct { rate,burst,tokens float64; at time.Time }
func newPacketPacer(rate,burst int,now time.Time)*packetPacer {
 if burst<1 {burst=32}
 return &packetPacer{float64(rate),float64(burst),float64(burst),now}
}
func(p *packetPacer) take(now time.Time)time.Duration {
 if p.rate<=0 {return 0}
 p.tokens=min(p.burst,p.tokens+max(0,now.Sub(p.at).Seconds())*p.rate);p.at=now
 if p.tokens>=1 {p.tokens--;return 0}
 return max(time.Microsecond,time.Duration((1-p.tokens)/p.rate*float64(time.Second)))
}
