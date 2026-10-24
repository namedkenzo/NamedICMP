package client

import (
 "context"
 "crypto/rand"
 "encoding/binary"
 "encoding/hex"
 "errors"
 "fmt"
 "net"
 "sync"
 "sync/atomic"
 "time"
 "namedicmp/internal/wire"
)

type packetConn interface {
 ReadFromIP([]byte)(int,*net.IPAddr,error)
 WriteToIP([]byte,*net.IPAddr)(int,error)
 Close()error
}
type Tunnel struct {
 conn packetConn
 peer *net.IPAddr
 security *wire.Security
 key int32
 id uint16
 seq atomic.Uint32
 mu sync.Mutex
 flows map[string]chan wire.Message
 probe chan wire.Message
 done chan struct{}
 once sync.Once
 Upload,Download atomic.Uint64
}
func randomID() string {b:=make([]byte,16);if _,e:=rand.Read(b);e!=nil{panic(e)};return hex.EncodeToString(b)}
func Open(ctx context.Context,p Profile) (*Tunnel,error) {
 ips,err:=net.DefaultResolver.LookupIP(ctx,"ip4",p.Host);if err!=nil || len(ips)==0{return nil,errors.New("cannot resolve server IPv4 address")}
 conn,err:=net.ListenIP("ip4:icmp",&net.IPAddr{IP:net.IPv4zero});if err!=nil{return nil,fmt.Errorf("raw ICMP socket: run namedP as Administrator: %w",err)}
 sec:=wire.NewSecurity();if err=sec.SetCredentials([]wire.Credential{p.Credential});err!=nil{conn.Close();return nil,err}
 b:=make([]byte,2);rand.Read(b)
 t:=&Tunnel{conn:conn,peer:&net.IPAddr{IP:ips[0]},security:sec,key:p.Credential.Key,id:binary.BigEndian.Uint16(b),flows:map[string]chan wire.Message{},probe:make(chan wire.Message,8),done:make(chan struct{})}
 conn.SetReadBuffer(4<<20);conn.SetWriteBuffer(4<<20)
 go t.read();go func(){select{case <-ctx.Done():t.Close();case <-t.done:}}()
 return t,nil
}
func (t *Tunnel) ServerIP() string{return t.peer.IP.String()}
func (t *Tunnel) Done() <-chan struct{}{return t.done}
func (t *Tunnel) Close(){t.once.Do(func(){close(t.done);t.conn.Close()})}
func (t *Tunnel) send(m wire.Message) error {
 m.Key=t.key;m.ReplyType=0;m.Timeout=60;m.Window=20000;m.ResendMS=200
 data,err:=t.security.Encode(m);if err!=nil{return err}
 _,err=t.conn.WriteToIP(wire.Echo(8,t.id,uint16(t.seq.Add(1)),data),t.peer);return err
}
func(t *Tunnel) read(){
 defer t.Close();b:=make([]byte,65535)
 for {n,peer,err:=t.conn.ReadFromIP(b);if err!=nil{return};if !peer.IP.Equal(t.peer.IP){continue}
 typ,id,_,payload,err:=wire.DecodeEcho(b[:n]);if err!=nil || typ!=0 || id!=t.id{continue}
 m,_,err:=t.security.Decode(peer.String(),id,payload);if err!=nil || m.ReplyType!=-1{continue};m.Payload=append([]byte(nil),m.Payload...)
 if m.Kind==1{select{case t.probe<-m:default:};continue}
 t.mu.Lock();ch:=t.flows[m.Connection];t.mu.Unlock();if ch!=nil{select{case ch<-m:default:}}
 }
}
func(t *Tunnel) Probe(ctx context.Context)error{
 token:=randomID();ticker:=time.NewTicker(500*time.Millisecond);defer ticker.Stop()
 for {if err:=t.send(wire.Message{Kind:1,Payload:[]byte(token)});err!=nil{return err}
 select{case <-ctx.Done():return errors.New("ICMP handshake timed out; check credentials and firewall");case <-t.done:return errors.New("ICMP transport closed");case m:=<-t.probe:if string(m.Payload)==token{return nil};case <-ticker.C:}
 }
}
func(t *Tunnel) register()(string,chan wire.Message,error){
 t.mu.Lock();defer t.mu.Unlock();if len(t.flows)>=256{return "",nil,errors.New("client flow limit reached")};id:=randomID();ch:=make(chan wire.Message,256);t.flows[id]=ch;return id,ch,nil
}
func(t *Tunnel) unregister(id,target string,tcp bool){t.mu.Lock();delete(t.flows,id);t.mu.Unlock();_ =t.send(wire.Message{Kind:2,Connection:id,Target:target,TCP:tcp})}
