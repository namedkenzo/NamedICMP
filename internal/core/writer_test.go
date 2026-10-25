package core

import (
 "context"
 "errors"
 "net"
 "path/filepath"
 "sync"
 "testing"
 "time"
 "namedicmp/internal/store"
 "namedicmp/internal/wire"
 "namedicmp/internal/outbound"
)

func TestPacketPacerBoundsBurstAndRate(t *testing.T) {
 now:=time.Unix(1,0);p:=newPacketPacer(100,2,now)
 if p.take(now)!=0||p.take(now)!=0 {t.Fatal("initial burst missing")}
 if delay:=p.take(now);delay!=10*time.Millisecond {t.Fatalf("unbounded rate: %v",delay)}
 if p.take(now.Add(10*time.Millisecond))!=0 {t.Fatal("token not refilled")}
 now=now.Add(time.Hour)
 if p.take(now)!=0||p.take(now)!=0||p.take(now)==0 {t.Fatal("idle time accumulated an unbounded burst")}
 if newPacketPacer(0,2,now).take(now)!=0 {t.Fatal("explicit disabled pacing not respected")}
}

type blockedWriterTransport struct { entered,release,done chan struct{}; once,closed sync.Once; out chan response }
func(t *blockedWriterTransport)Read()(Packet,error){<-t.done;return Packet{},net.ErrClosed}
func(t *blockedWriterTransport)Close()error{t.closed.Do(func(){close(t.done)});return nil}
func(t *blockedWriterTransport)Write(p Packet,m wire.Message)error {
 t.once.Do(func(){close(t.entered)})
 select{case <-t.release:case <-t.done:return net.ErrClosed}
 select{case t.out<-response{p,m}:return nil;case <-t.done:return net.ErrClosed}
}
func TestSaturatedWriterDoesNotBlockReceiveOrStarveProbes(t *testing.T){
 db,err:=store.Open(filepath.Join(t.TempDir(),"state.json"));if err!=nil{t.Fatal(err)};defer db.Close()
 user,err:=db.PutUser(store.User{Name:"load",Enabled:true});if err!=nil{t.Fatal(err)}
 transport:=&blockedWriterTransport{entered:make(chan struct{}),release:make(chan struct{}),done:make(chan struct{}),out:make(chan response,64)}
 conf:=store.DefaultConfig();server,err:=New(db,conf,transport);if err!=nil{t.Fatal(err)};defer server.Stop()
 packet:=Packet{Source:"192.0.2.1",Message:wire.Message{Key:user.Key}}
 queued:=make(chan struct{})
 go func(){server.reply(packet,wire.Message{Kind:0,Payload:[]byte("busy")});close(queued)}()
 select{case <-queued:case <-time.After(time.Second):t.Fatal("data write blocked its caller")}
 select{case <-transport.entered:case <-time.After(time.Second):t.Fatal("writer never started")}
 for i:=0;i<3000;i++{server.reply(packet,wire.Message{Kind:0,Payload:[]byte("queued")})}
 returned:=make(chan struct{})
 go func(){ping:=packet;ping.Message.Kind=1;ping.Message.Payload=[]byte("probe");server.Handle(context.Background(),ping);close(returned)}()
 select{case <-returned:case <-time.After(time.Second):t.Fatal("receive loop blocked behind saturated output")}
 if server.writeQueueDrops.Load()==0 {t.Fatal("queue was not bounded")}
 close(transport.release)
 timer:=time.NewTimer(time.Second);defer timer.Stop()
 for i:=0;i<3;i++ {select{case answer:=<-transport.out:if answer.m.Kind==1 {return};case <-timer.C:t.Fatal("probe starved by bulk traffic")}}
 t.Fatal("control reply was not prioritized")
}

func TestOutboundFailureCategoriesDoNotLeakTargets(t *testing.T){
 c:=&Core{}
 for _,err:=range []error{outbound.ErrResolve,context.DeadlineExceeded,&net.OpError{Op:"dial",Err:errors.New("refused")},errors.New("proxy rejected")} {c.recordDialError(err)}
 if c.dialErrors.Load()!=4||c.outboundDNS.Load()!=1||c.outboundTimeout.Load()!=1||c.outboundNetwork.Load()!=1||c.outboundOther.Load()!=1 {t.Fatal("failure categories incorrect")}
}
