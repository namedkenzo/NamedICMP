package core

import (
 "encoding/binary"
 "net"
 "sync"
 "time"
 "golang.org/x/sys/unix"
 "namedicmp/internal/wire"
)

// Opt-in AF_PACKET/SOCK_DGRAM returns network headers without link headers.
// BPF limits reception to ICMP/ICMPv6; user-space checks destination ownership,
// fragments/checksums, Echo framing and the same user credentials as raw IP.
// This is not a remote firewall bypass and does not alter conntrack rules.
type packetReader struct {mu sync.RWMutex;fd int;sockets map[int]*RawTransport;locals map[string]bool;sampled time.Time;buffer []byte}
func openPacketReader(sockets map[int]*RawTransport)(*packetReader,error){
 fd,err:=unix.Socket(unix.AF_PACKET,unix.SOCK_DGRAM|unix.SOCK_CLOEXEC,int(htons(unix.ETH_P_ALL)));if err!=nil{return nil,err}
 reader:=&packetReader{fd:fd,sockets:sockets,buffer:make([]byte,65535)}
 fail:=func(err error)(*packetReader,error){_ =reader.Close();return nil,err}
 if err=unix.SetsockoptTimeval(fd,unix.SOL_SOCKET,unix.SO_RCVTIMEO,&unix.Timeval{Usec:250000});err!=nil{return fail(err)}
 _ =unix.SetsockoptInt(fd,unix.SOL_SOCKET,unix.SO_RCVBUF,4<<20)
 filters:=[]unix.SockFilter{
  {Code:unix.BPF_LD|unix.BPF_B|unix.BPF_ABS,K:0},
  {Code:unix.BPF_ALU|unix.BPF_AND|unix.BPF_K,K:0xf0},
  {Code:unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K,K:0x40,Jf:3},
  {Code:unix.BPF_LD|unix.BPF_B|unix.BPF_ABS,K:9},
  {Code:unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K,K:1,Jt:4,Jf:5},
  {Code:unix.BPF_RET|unix.BPF_K,K:0},
  {Code:unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K,K:0x60,Jf:3},
  {Code:unix.BPF_LD|unix.BPF_B|unix.BPF_ABS,K:6},
  {Code:unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K,K:58,Jf:1},
  {Code:unix.BPF_RET|unix.BPF_K,K:65535},
  {Code:unix.BPF_RET|unix.BPF_K,K:0},
 }
 if err=unix.SetsockoptSockFprog(fd,unix.SOL_SOCKET,unix.SO_ATTACH_FILTER,&unix.SockFprog{Len:uint16(len(filters)),Filter:&filters[0]});err!=nil{return fail(err)}
 return reader,nil
}
func htons(value uint16)uint16{return value<<8|value>>8}
func(r *packetReader)Close()error{r.mu.Lock();defer r.mu.Unlock();if r.fd<0{return nil};fd:=r.fd;r.fd=-1;return unix.Close(fd)}
func(r *packetReader)owns(destination net.IP,family int)bool{
 socket:=r.sockets[family];if socket==nil{return false}
 bound:=socket.c.LocalAddr().(*net.IPAddr).IP
 if bound!=nil&&!bound.IsUnspecified(){return bound.Equal(destination)}
 if time.Since(r.sampled)>5*time.Second||r.locals==nil{
  r.locals=map[string]bool{};addresses,_:=net.InterfaceAddrs();for _,address:=range addresses{ip,_,err:=net.ParseCIDR(address.String());if err==nil{r.locals[ip.String()]=true}};r.sampled=time.Now()
 }
 return r.locals[destination.String()]
}
func(r *packetReader)Read()(Packet,error){
 buffer:=r.buffer
 for{
  r.mu.RLock();if r.fd<0{r.mu.RUnlock();return Packet{},net.ErrClosed};n,from,err:=unix.Recvfrom(r.fd,buffer,0);r.mu.RUnlock()
  if err==unix.EAGAIN||err==unix.EWOULDBLOCK||err==unix.EINTR{continue};if err!=nil{return Packet{},err}
  link,ok:=from.(*unix.SockaddrLinklayer);if !ok||link.Pkttype!=unix.PACKET_HOST{continue}
  frame:=buffer[:n];if n<20{continue}
  family:=4;var source,destination net.IP;decode:=wire.DecodeEcho
  if frame[0]>>4==4{
   total:=int(binary.BigEndian.Uint16(frame[2:4]));if total<20||total>n{continue};frame=frame[:total]
   ihl:=int(frame[0]&15)*4;if ihl<20||ihl>len(frame)||wire.Checksum(frame[:ihl])!=0{continue}
   source,destination=net.IP(frame[12:16]),net.IP(frame[16:20])
  }else if frame[0]>>4==6&&n>=48{
   family=6;source,destination=net.IP(frame[8:24]),net.IP(frame[24:40]);decode=wire.DecodeEcho6
   if frame[6]!=58||int(binary.BigEndian.Uint16(frame[4:6]))!=n-40||wire.Checksum6(source,destination,frame[40:])!=0{continue}
  }else{continue}
  if source.IsUnspecified()||source.IsMulticast()||!r.owns(destination,family){continue}
  _,id,seq,payload,err:=decode(frame);if err!=nil{continue}
  message,mode,err:=r.sockets[family].security.Decode(source.String(),id,payload)
  if err!=nil||message.ReplyType!=0&&message.ReplyType!=8{continue}
  if mode=="none"{message.Payload=append([]byte(nil),message.Payload...)}
  return Packet{Source:source.String(),Family:family,EchoID:id,EchoSeq:seq,Message:message,Encryption:mode},nil
 }
}
