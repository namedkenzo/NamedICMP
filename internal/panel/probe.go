package panel

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"namedicmp/internal/wire"
)

type echoResult struct { sent bool; rtt time.Duration; err error }
type probeSample struct { received bool; ms float64 }
type probeSnapshot struct {
	Family string `json:"family"`
	Target string `json:"target"`
	Status string `json:"status"`
	Error string `json:"error,omitempty"`
	LastAt int64 `json:"last_at"`
	LastSuccess int64 `json:"last_success"`
	Sent int `json:"sent"`
	Received int `json:"received"`
	TotalSent uint64 `json:"total_sent"`
	TotalReceived uint64 `json:"total_received"`
	Loss *float64 `json:"loss_percent"`
	RTT *float64 `json:"rtt_ms"`
	Average *float64 `json:"average_ms"`
	Minimum *float64 `json:"minimum_ms"`
	Maximum *float64 `json:"maximum_ms"`
	Jitter *float64 `json:"jitter_ms"`
}
type pathProbe struct {
	mu sync.Mutex
	family, target, status, problem string
	enabled bool
	lastAt, lastSuccess int64
	totalSent, totalReceived uint64
	samples []probeSample
}
func newPathProbe(family,target string,enabled bool) *pathProbe {
	status:="disabled";if enabled {status="waiting"}
	return &pathProbe{family:family,target:target,enabled:enabled,status:status}
}
func (p *pathProbe) record(now time.Time, result echoResult) {
	p.mu.Lock();defer p.mu.Unlock()
	p.lastAt=now.UnixMilli();p.problem=""
	if result.err!=nil || !result.sent {
		p.status="unavailable"
		if result.err!=nil {p.problem=result.err.Error()}
		return // A failed send or unavailable socket is not network loss.
	}
	p.totalSent++
	sample:=probeSample{received:result.rtt>0,ms:float64(result.rtt)/float64(time.Millisecond)}
	p.status="timeout"
	if sample.received {p.totalReceived++;p.lastSuccess=p.lastAt;p.status="ok"}
	if len(p.samples)==30 {copy(p.samples,p.samples[1:]);p.samples=p.samples[:29]}
	p.samples=append(p.samples,sample)
}
func numberPointer(n float64) *float64 {return &n}
func (p *pathProbe) snapshot() probeSnapshot {
	p.mu.Lock();defer p.mu.Unlock()
	s:=probeSnapshot{Family:p.family,Target:p.target,Status:p.status,Error:p.problem,LastAt:p.lastAt,LastSuccess:p.lastSuccess,Sent:len(p.samples),TotalSent:p.totalSent,TotalReceived:p.totalReceived}
	var sum,minimum,maximum,jitter,previous float64
	var consecutive bool
	pairs:=0
	for _,sample:=range p.samples {
		if !sample.received {consecutive=false;continue}
		if s.Received==0 {minimum=sample.ms}
		s.Received++;sum+=sample.ms;minimum=math.Min(minimum,sample.ms);maximum=math.Max(maximum,sample.ms)
		if consecutive {jitter+=math.Abs(sample.ms-previous);pairs++}
		previous=sample.ms;consecutive=true
	}
	if s.Sent>0 {s.Loss=numberPointer(100*float64(s.Sent-s.Received)/float64(s.Sent))}
	if s.Received>0 {s.Average=numberPointer(sum/float64(s.Received));s.Minimum=numberPointer(minimum);s.Maximum=numberPointer(maximum)}
	if pairs>0 {s.Jitter=numberPointer(jitter/float64(pairs))}
	if p.status=="ok"&&len(p.samples)>0 {s.RTT=numberPointer(p.samples[len(p.samples)-1].ms)}
	return s
}
func (p *pathProbe) run(ctx context.Context) {
	if !p.enabled {return}
	for {
		result:=echoOnce(ctx,p.family,p.target)
		if ctx.Err()!=nil {return}
		p.record(time.Now(),result)
		timer:=time.NewTimer(10*time.Second)
		select {case <-ctx.Done():timer.Stop();return;case <-timer.C:}
	}
}

// Only reply packets reach this socket; a tunnel under load must not turn a
// once-per-10-second path check into a second full traffic decoder.
func echoOnce(ctx context.Context,family,target string) echoResult {
	network,bind,kind:="ip4:icmp","0.0.0.0",byte(8)
	if family=="ipv6" {network,bind,kind="ip6:ipv6-icmp","::",128}
	conn,err:=net.ListenIP(network,&net.IPAddr{IP:net.ParseIP(bind)})
	if err!=nil {return echoResult{err:err}}
	defer conn.Close()
	stop:=context.AfterFunc(ctx,func(){_ = conn.Close()});defer stop()
	raw,err:=conn.SyscallConn();if err!=nil {return echoResult{err:err}}
	var filterErr error
	err=raw.Control(func(fd uintptr){
		if family=="ipv6" {
			filter:=unix.ICMPv6Filter{}
			for i:=range filter.Data {filter.Data[i]=^uint32(0)}
			filter.Data[129/32] &^= 1 << (129%32)
			filterErr=unix.SetsockoptICMPv6Filter(int(fd),unix.IPPROTO_ICMPV6,unix.ICMP6_FILTER,&filter)
		} else {filterErr=unix.SetsockoptInt(int(fd),unix.IPPROTO_RAW,1,int(uint32(0xfffffffe)))}
	})
	if err!=nil {return echoResult{err:err}};if filterErr!=nil {return echoResult{err:filterErr}}
	_ = conn.SetReadBuffer(32*1024)
	if err=conn.SetDeadline(time.Now().Add(2*time.Second));err!=nil {return echoResult{err:err}}
	var nonce [32]byte
	if _,err=rand.Read(nonce[:]);err!=nil {return echoResult{err:err}}
	id,seq:=binary.BigEndian.Uint16(nonce[:2]),binary.BigEndian.Uint16(nonce[2:4])
	packet:=wire.Echo(kind,id,seq,nonce[:])
	if family=="ipv6" {packet=wire.Echo6(kind,id,seq,nonce[:])}
	peer:=&net.IPAddr{IP:net.ParseIP(target)}
	start:=time.Now()
	if _,err=conn.WriteToIP(packet,peer);err!=nil {return echoResult{err:err}}
	buf:=make([]byte,2048)
	for {
		n,source,readErr:=conn.ReadFromIP(buf)
		if readErr!=nil {
			var timeout net.Error
			if errors.As(readErr,&timeout)&&timeout.Timeout() {return echoResult{sent:true}}
			return echoResult{err:readErr}
		}
		if !source.IP.Equal(peer.IP) {continue}
		var typ byte;var gotID,gotSeq uint16;var payload []byte
		if family=="ipv6" {typ,gotID,gotSeq,payload,err=wire.DecodeEcho6(buf[:n])} else {typ,gotID,gotSeq,payload,err=wire.DecodeEcho(buf[:n])}
		if err==nil&&((family=="ipv6"&&typ==129)||(family=="ipv4"&&typ==0))&&gotID==id&&gotSeq==seq&&bytes.Equal(payload,nonce[:]) {return echoResult{sent:true,rtt:time.Since(start)}}
	}
}
