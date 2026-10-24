package client

import (
 "context"
 "encoding/binary"
 "errors"
 "fmt"
 "io"
 "net"
 "strconv"
 "sync"
 "time"
 "namedicmp/internal/wire"
)

// ServeSOCKS binds only loopback. UDP lifetime is tied to its TCP control
// stream, and destination flows have independent connection IDs.
func(t *Tunnel) ServeSOCKS(ctx context.Context,port int,ready func())error{
 l,err:=net.Listen("tcp4",net.JoinHostPort("127.0.0.1",strconv.Itoa(port)));if err!=nil{return err};defer l.Close()
 stop:=make(chan struct{});defer close(stop);go func(){select{case <-ctx.Done():case <-t.done:case <-stop:return};l.Close()}()
 slots:=make(chan struct{},128);var wg sync.WaitGroup;defer wg.Wait();ready()
 for{c,e:=l.Accept();if e!=nil{return e};select{case slots<-struct{}{}:wg.Add(1);go func(){defer wg.Done();defer func(){<-slots}();defer c.Close();t.socks(ctx,c)}();default:c.Close()}}
}
func readAddress(r io.Reader,kind byte)(string,error){
 var host string
 switch kind{case 1:b:=make([]byte,4);if _,e:=io.ReadFull(r,b);e!=nil{return "",e};host=net.IP(b).String()
 case 3:var n [1]byte;if _,e:=io.ReadFull(r,n[:]);e!=nil{return "",e};if n[0]==0{return "",errors.New("empty host")};b:=make([]byte,int(n[0]));if _,e:=io.ReadFull(r,b);e!=nil{return "",e};host=string(b)
 case 4:b:=make([]byte,16);if _,e:=io.ReadFull(r,b);e!=nil{return "",e};host=net.IP(b).String()
 default:return "",errors.New("unsupported address")}
 var port [2]byte;if _,e:=io.ReadFull(r,port[:]);e!=nil{return "",e};return net.JoinHostPort(host,strconv.Itoa(int(binary.BigEndian.Uint16(port[:])))),nil
}
func(t *Tunnel) socks(ctx context.Context,c net.Conn){
 c.SetDeadline(time.Now().Add(10*time.Second));var h [2]byte;if _,e:=io.ReadFull(c,h[:]);e!=nil || h[0]!=5{return};methods:=make([]byte,int(h[1]));if _,e:=io.ReadFull(c,methods);e!=nil{return};ok:=false;for _,m:=range methods{ok=ok||m==0};if !ok{writeAll(c,[]byte{5,255});return};if writeAll(c,[]byte{5,0})!=nil{return}
 var req [4]byte;if _,e:=io.ReadFull(c,req[:]);e!=nil || req[0]!=5 || req[2]!=0{return};target,e:=readAddress(c,req[3]);if e!=nil{return};c.SetDeadline(time.Time{})
 reply:=func(code byte,port int)error{return writeAll(c,[]byte{5,code,0,1,127,0,0,1,byte(port>>8),byte(port)})}
 switch req[1]{case 1:connected:=false;err:=t.Stream(ctx,c,target,func()error{connected=true;return reply(0,0)});if err!=nil && !connected{reply(5,0)}
 case 3:t.associate(ctx,c,reply)
 default:reply(7,0)}
}
func(t *Tunnel) associate(ctx context.Context,c net.Conn,reply func(byte,int)error){
 udp,e:=net.ListenUDP("udp4",&net.UDPAddr{IP:net.IPv4(127,0,0,1)});if e!=nil{reply(1,0);return};defer udp.Close()
 if reply(0,udp.LocalAddr().(*net.UDPAddr).Port)!=nil{return}
 assoc,cancel:=context.WithCancel(ctx);defer cancel();go func(){io.Copy(io.Discard,c);cancel();udp.Close()}()
 stop:=make(chan struct{});defer close(stop);go func(){select{case <-assoc.Done():case <-t.done:case <-stop:return};udp.Close();c.Close()}()
 type flow struct{id string;in chan wire.Message;last time.Time; done chan struct{}}
 flows:=map[string]*flow{};defer func(){for target,f:=range flows{close(f.done);t.unregister(f.id,target,false)}}()
 var peer *net.UDPAddr;buf:=make([]byte,8500)
 // Replies are multiplexed into this one association goroutine; no unbounded
 // per-packet goroutines and no cross-client datagram delivery.
 responses:=make(chan wire.Message,256)
 type datagram struct{b []byte;src *net.UDPAddr;err error}
 packets:=make(chan datagram,256)
 go func(){for{n,src,err:=udp.ReadFromUDP(buf);p:=datagram{append([]byte(nil),buf[:n]...),src,err};select{case packets<-p:case <-assoc.Done():return};if err!=nil{return}}}()
 tick:=time.NewTicker(5*time.Second);defer tick.Stop()
 for{var packet datagram
 select{case <-assoc.Done():return;case <-t.done:return
 case <-tick.C:now:=time.Now();for k,f:=range flows{if now.Sub(f.last)>time.Minute{close(f.done);t.unregister(f.id,k,false);delete(flows,k)}};continue
 case m:=<-responses:
 if peer==nil || m.TCP || m.Kind!=0 || len(m.Payload)>8192{continue};f:=flows[m.Target];if f==nil || f.id!=m.Connection{continue};header,e:=udpHeader(m.Target);if e!=nil{continue};if _,e=udp.WriteToUDP(append(header,m.Payload...),peer);e==nil{t.Download.Add(uint64(len(m.Payload)));f.last=time.Now()};continue
 case packet=<-packets:}
 n,src,err:=len(packet.b),packet.src,packet.err;data:=packet.b
 if err==nil{
 if !src.IP.IsLoopback() || peer!=nil && src.String()!=peer.String(){continue};if n<4 || data[0]!=0 || data[1]!=0 || data[2]!=0{continue}
 cursor:=&sliceReader{b:data[4:n]};target,e:=readAddress(cursor,data[3]);if e!=nil || len(cursor.b)>8192{continue};if peer==nil{peer=src}
 now:=time.Now()
 f:=flows[target];if f==nil{if len(flows)>=128{continue};id,ch,e:=t.register();if e!=nil{continue};f=&flow{id:id,in:ch,last:now,done:make(chan struct{})};flows[target]=f
 go func(ch chan wire.Message,done chan struct{}){for{select{case <-assoc.Done():return;case <-done:return;case <-t.done:return;case m:=<-ch:select{case responses<-m:case <-assoc.Done():return;case <-done:return}}}}(ch,f.done)
 };f.last=now
 if t.send(wire.Message{Connection:f.id,Target:target,Payload:append([]byte(nil),cursor.b...)})==nil{t.Upload.Add(uint64(len(cursor.b)))}
 } else if !isTimeout(err){return}
 select{case <-assoc.Done():return;case <-t.done:return;default:}
 }
}
func isTimeout(e error)bool{n,ok:=e.(net.Error);return ok&&n.Timeout()}
type sliceReader struct{b []byte}
func(r *sliceReader)Read(p []byte)(int,error){if len(r.b)==0{return 0,io.EOF};n:=copy(p,r.b);r.b=r.b[n:];return n,nil}
func udpHeader(target string)([]byte,error){host,p,e:=net.SplitHostPort(target);if e!=nil{return nil,e};port,e:=strconv.Atoi(p);if e!=nil{return nil,e};b:=[]byte{0,0,0};ip:=net.ParseIP(host);switch{case ip.To4()!=nil:b=append(b,1);b=append(b,ip.To4()...);case ip!=nil:b=append(b,4);b=append(b,ip.To16()...);case len(host)<=255:b=append(b,3,byte(len(host)));b=append(b,host...);default:return nil,fmt.Errorf("invalid UDP target")};return append(b,byte(port>>8),byte(port)),nil}
