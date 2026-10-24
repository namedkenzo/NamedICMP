//go:build !windows

package client

import (
 "bytes"
 "context"
 "encoding/binary"
 "fmt"
 "io"
 "net"
 "path/filepath"
 "sync"
 "sync/atomic"
 "testing"
 "time"
 "namedicmp/internal/core"
 "namedicmp/internal/store"
 "namedicmp/internal/wire"
)

// In-memory ICMP link exercises the real server, codecs, crypto, reliability
// and SOCKS sockets without raw-socket privileges or an external VPS.
type testLink struct{
 server *core.Core
 security *wire.Security
 replies chan []byte
 done chan struct{}
 once sync.Once
 drop atomic.Bool
}
func(l *testLink) Close()error{l.once.Do(func(){close(l.done)});return nil}
func(l *testLink) Read()(core.Packet,error){<-l.done;return core.Packet{},net.ErrClosed}
func(l *testLink) SetCredentials(c []wire.Credential)error{return l.security.SetCredentials(c)}
func(l *testLink) Write(p core.Packet,m wire.Message)error{
 if m.TCP {f,e:=wire.DecodeFrame(m.Payload);if e==nil && f.Kind==0 && f.DataKind==0 && l.drop.CompareAndSwap(true,false){return nil}}
 b,e:=l.security.Encode(m);if e!=nil{return e};b=wire.Echo(0,p.EchoID,p.EchoSeq,b)
 select{case l.replies<-b:return nil;case <-l.done:return net.ErrClosed}
}
func(l *testLink) WriteToIP(b []byte,_ *net.IPAddr)(int,error){
 _,id,seq,p,e:=wire.DecodeEcho(b);if e!=nil{return 0,e};m,mode,e:=l.security.Decode("192.0.2.10",id,p);if e!=nil{return 0,e}
 l.server.Handle(context.Background(),core.Packet{Source:"192.0.2.10",EchoID:id,EchoSeq:seq,Message:m,Encryption:mode});return len(b),nil
}
func(l *testLink) ReadFromIP(b []byte)(int,*net.IPAddr,error){select{case p:=<-l.replies:return copy(b,p),&net.IPAddr{IP:net.ParseIP("192.0.2.1")},nil;case <-l.done:return 0,nil,net.ErrClosed}}
func newTestTunnel(t *testing.T,mode string)*Tunnel{
 t.Helper();db,e:=store.Open(filepath.Join(t.TempDir(),"state.json"));if e!=nil{t.Fatal(e)}
 u,e:=db.PutUser(store.User{Name:"client",Enabled:true,Encryption:mode,Quota:8<<20});if e!=nil{t.Fatal(e)}
 link:=&testLink{security:wire.NewSecurity(),replies:make(chan []byte,4096),done:make(chan struct{})};link.drop.Store(true)
 conf:=store.DefaultConfig();conf.AllowPrivate=true;srv,e:=core.New(db,conf,link);if e!=nil{t.Fatal(e)};link.server=srv
 sec:=wire.NewSecurity();if e=sec.SetCredentials([]wire.Credential{{Key:u.Key,Mode:u.Encryption,Secret:u.EncryptionKey}});e!=nil{t.Fatal(e)}
 tunnel:=&Tunnel{conn:link,peer:&net.IPAddr{IP:net.ParseIP("192.0.2.1")},security:sec,key:u.Key,id:4321,flows:map[string]chan wire.Message{},probe:make(chan wire.Message,8),done:make(chan struct{})}
 go tunnel.read();t.Cleanup(func(){tunnel.Close();srv.Stop();db.Close()});return tunnel
}
func TestClientServerHandshakeAndTCPRetransmission(t *testing.T){
 for _,mode:=range []string{"none","aes128","aes256","chacha20"}{t.Run(mode,func(t *testing.T){
 tunnel:=newTestTunnel(t,mode);ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel();if e:=tunnel.Probe(ctx);e!=nil{t.Fatal(e)}
 target,e:=net.Listen("tcp4","127.0.0.1:0");if e!=nil{t.Fatal(e)};defer target.Close()
 go func(){c,e:=target.Accept();if e==nil{defer c.Close();io.Copy(c,c)}}()
 local,e:=net.Listen("tcp4","127.0.0.1:0");if e!=nil{t.Fatal(e)};defer local.Close();result:=make(chan error,1)
 go func(){c,e:=local.Accept();if e!=nil{result<-e;return};result<-tunnel.Stream(ctx,c,target.Addr().String(),func()error{return nil})}()
 c,e:=net.Dial("tcp4",local.Addr().String());if e!=nil{t.Fatal(e)};defer c.Close();c.SetDeadline(time.Now().Add(8*time.Second))
 payload:=bytes.Repeat([]byte("namedP TCP integrity "),2048)
 written:=make(chan error,1);go func(){written<-writeAll(c,payload)}()
 got:=make([]byte,len(payload));if _,e=io.ReadFull(c,got);e!=nil{t.Fatal(e)};if e=<-written;e!=nil{t.Fatal(e)};if !bytes.Equal(got,payload){t.Fatal("corrupt stream")}
 c.(*net.TCPConn).CloseWrite();if _,e=io.Copy(io.Discard,c);e!=nil{t.Fatal(e)}
 select{case e=<-result:if e!=nil{t.Fatal(e)};case <-ctx.Done():t.Fatal("stream did not half-close")}
 if tunnel.Download.Load()!=uint64(len(payload)){t.Fatal("incorrect unique-byte accounting")}
 })}
}
func TestSOCKSUDPAssociationMultipleDestinations(t *testing.T){
 tunnel:=newTestTunnel(t,"aes256");ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 // Reserve a free port, then let the actual SOCKS server bind it.
 reserve,e:=net.Listen("tcp4","127.0.0.1:0");if e!=nil{t.Fatal(e)};port:=reserve.Addr().(*net.TCPAddr).Port;reserve.Close()
 ready:=make(chan struct{});go tunnel.ServeSOCKS(ctx,port,func(){close(ready)});select{case <-ready:case <-time.After(2*time.Second):t.Fatal("SOCKS startup")}
 control,e:=net.Dial("tcp4",fmt.Sprintf("127.0.0.1:%d",port));if e!=nil{t.Fatal(e)};defer control.Close();control.SetDeadline(time.Now().Add(5*time.Second))
 writeAll(control,[]byte{5,1,0});var method [2]byte;io.ReadFull(control,method[:]);if method!=[2]byte{5,0}{t.Fatal("SOCKS method")}
 writeAll(control,[]byte{5,3,0,1,0,0,0,0,0,0});var reply [10]byte;if _,e=io.ReadFull(control,reply[:]);e!=nil || reply[1]!=0{t.Fatal("UDP association",e)}
 udp,e:=net.DialUDP("udp4",nil,&net.UDPAddr{IP:net.IPv4(127,0,0,1),Port:int(binary.BigEndian.Uint16(reply[8:]))});if e!=nil{t.Fatal(e)};defer udp.Close();udp.SetDeadline(time.Now().Add(5*time.Second))
 for i:=0;i<2;i++{target,e:=net.ListenUDP("udp4",&net.UDPAddr{IP:net.IPv4(127,0,0,1)});if e!=nil{t.Fatal(e)};defer target.Close()
 go func(){b:=make([]byte,2048);n,peer,e:=target.ReadFromUDP(b);if e==nil{target.WriteToUDP(b[:n],peer)}}()
 header,_:=udpHeader(target.LocalAddr().String());payload:=[]byte(fmt.Sprintf("datagram-%d",i));udp.Write(append(header,payload...));got:=make([]byte,2048);n,e:=udp.Read(got);if e!=nil{t.Fatal(e)};if !bytes.Equal(got[:n],append(header,payload...)){t.Fatal("mixed UDP destination")}
 }
}
