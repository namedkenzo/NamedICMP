package client

import (
 "context"
 "errors"
 "io"
 "net"
 "time"
 "namedicmp/internal/reliable"
 "namedicmp/internal/wire"
)

type chunk struct {b []byte;err error}
// Stream owns its reliability state in a single goroutine. Retries do not
// charge application counters again. The local socket uses TCP half-close.
func(t *Tunnel) Stream(ctx context.Context,c net.Conn,target string,ready func()error)error{
 id,in,err:=t.register();if err!=nil{return err};defer t.unregister(id,target,true)
 defer c.Close();r:=reliable.NewReliable(20000,200*time.Millisecond);_ =r.Queue(1,nil)
 send:=func(f wire.Frame)error{return t.send(wire.Message{Connection:id,Target:target,TCP:true,Payload:wire.EncodeFrame(f)})}
 reads:=make(chan chunk,1);credit:=make(chan struct{},1);stop:=make(chan struct{});defer close(stop)
 connected,eof,remoteEOF,reading,closeSent:=false,false,false,false,false
 last:=time.Now();started:=last;var pending []byte;var closeAt time.Time
 timer:=time.NewTimer(time.Hour);defer timer.Stop()
 go func(){for{select{case <-stop:return;case <-credit:};b:=make([]byte,wire.PayloadSize);n,e:=c.Read(b);select{case reads<-chunk{b[:n],e}:case <-stop:return};if e!=nil{return}}}()
 for{
 now:=time.Now();if !connected && now.Sub(started)>12*time.Second{return errors.New("target connection timed out")};if now.Sub(last)>time.Minute{return errors.New("stream idle timeout")}
 if pending!=nil && r.Space()>0{_ =r.Queue(0,pending);t.Upload.Add(uint64(len(pending)));pending=nil;last=now}
 if connected && !eof && !reading && pending==nil && r.Space()>1{credit<-struct{}{};reading=true}
 if eof && pending==nil && r.Space()>0 && !closeSent{_ =r.Queue(3,nil);closeSent=true;closeAt=now}
 if closeSent && (remoteEOF && r.Empty() || now.Sub(closeAt)>30*time.Second){return nil}
 for _,f:=range r.Due(now){if err:=send(f);err!=nil{return err}}
 if !timer.Stop(){select{case <-timer.C:default:}};timer.Reset(max(time.Millisecond,min(time.Second,r.Next(now))))
 select{
 case <-ctx.Done():return ctx.Err()
 case <-t.done:return errors.New("ICMP transport closed")
 case <-timer.C:
 case part:=<-reads:reading=false;pending=part.b;if part.err!=nil{eof=true}
 case m:=<-in:
 if m.Kind==2{if closeSent && remoteEOF{return nil};return errors.New("server closed the stream")};if !m.TCP{continue}
 f,e:=wire.DecodeFrame(m.Payload);if e!=nil || f.Compressed || len(f.Data)>wire.PayloadSize{return errors.New("invalid server frame")}
 controls,frames:=r.Input(f,time.Now());for _,ctl:=range controls{if e:=send(ctl);e!=nil{return e}}
 for _,v:=range frames{switch v.DataKind{
 case 2:if !connected{if e:=ready();e!=nil{return e};connected=true;last=time.Now()}
 case 0:if !connected || remoteEOF{return errors.New("unexpected server data")};c.SetWriteDeadline(time.Now().Add(10*time.Second));if e:=writeAll(c,v.Data);e!=nil{return e};c.SetWriteDeadline(time.Time{});t.Download.Add(uint64(len(v.Data)));last=time.Now()
 case 3:remoteEOF=true;if tcp,ok:=c.(interface{CloseWrite()error});ok{tcp.CloseWrite()}else{return nil}
 case 4:
 default:return errors.New("unexpected server control")
 }}
 }
 }
}
func writeAll(w io.Writer,b []byte)error{for len(b)>0{n,e:=w.Write(b);if e!=nil{return e};if n==0{return io.ErrShortWrite};b=b[n:]};return nil}
