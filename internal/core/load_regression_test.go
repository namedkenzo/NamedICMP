package core

import (
 "bytes"
 "context"
 "fmt"
 "io"
 "net"
 "path/filepath"
 "testing"
 "time"
 "namedicmp/internal/store"
 "namedicmp/internal/wire"
)

type lossyBulkTransport struct { *mockTransport; dataPackets int }
func(t *lossyBulkTransport)Write(p Packet,m wire.Message)error {
 if m.Kind==0 && m.TCP {if f,e:=wire.DecodeFrame(m.Payload);e==nil&&f.Kind==0&&f.DataKind==0 {
  t.dataPackets++;if t.dataPackets%23==0 {return nil}
 }}
 return t.mockTransport.Write(p,m)
}

// Bulk echo transfers stress the server, not a public speed-test service. A
// controlled 1/23 data loss verifies retransmission, integrity and probe liveness.
func TestParallelBulkTransfersKeepProbesAlive(t *testing.T){
 listener,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{t.Fatal(err)};defer listener.Close()
 go func(){for {conn,e:=listener.Accept();if e!=nil{return};go func(){defer conn.Close();_,_=io.Copy(conn,conn)}()}}()
 db,err:=store.Open(filepath.Join(t.TempDir(),"state.json"));if err!=nil{t.Fatal(err)};defer db.Close()
 user,err:=db.PutUser(store.User{Name:"bulk-load",Enabled:true});if err!=nil{t.Fatal(err)}
 transport:=&lossyBulkTransport{mockTransport:&mockTransport{out:make(chan response,8192),done:make(chan struct{})}}
 conf:=store.DefaultConfig();conf.AllowPrivate=true
 server,err:=New(db,conf,transport);if err!=nil{t.Fatal(err)};defer server.Stop()
 type flow struct { reliable *Reliable; expected,received []byte; offset int; connected bool }
 flows:=map[string]*flow{}
 for i:=0;i<8;i++ {f:=&flow{reliable:NewReliable(20000,100*time.Millisecond),expected:bytes.Repeat([]byte{byte(i+1)},128*1024)};_ =f.reliable.Queue(1,nil);flows[fmt.Sprintf("bulk-%d",i)]=f}
 send:=func(id string,frame wire.Frame){server.Handle(context.Background(),Packet{Source:"192.0.2.1",Message:wire.Message{Connection:id,Target:listener.Addr().String(),Key:user.Key,TCP:true,ReplyType:0,Window:20000,ResendMS:100,Timeout:60,Payload:wire.EncodeFrame(frame)}})}
 tick:=time.NewTicker(5*time.Millisecond);defer tick.Stop()
 timeout:=time.NewTimer(30*time.Second);defer timeout.Stop()
 probes:=map[string]time.Time{};lastProbe:=time.Now();probeReplies:=0
 for {
  completed:=0;now:=time.Now()
  for id,f:=range flows {
   if len(f.received)==len(f.expected){completed++}
   for f.connected&&f.offset<len(f.expected)&&f.reliable.Space()>0 {end:=min(f.offset+wire.PayloadSize,len(f.expected));_ =f.reliable.Queue(0,f.expected[f.offset:end]);f.offset=end}
   for _,frame:=range f.reliable.Due(now){send(id,frame)}
  }
  if completed==len(flows) {break}
  if now.Sub(lastProbe)>=300*time.Millisecond {
   token:=fmt.Sprintf("probe-%d",now.UnixNano());probes[token]=now;lastProbe=now
   server.Handle(context.Background(),Packet{Source:"192.0.2.1",Message:wire.Message{Kind:1,Key:user.Key,Payload:[]byte(token)}})
  }
  select {
  case <-timeout.C:t.Fatal("parallel transfer stalled or disconnected")
  case <-tick.C:
  case answer:=<-transport.out:
   if answer.m.Kind==1 {if sent,ok:=probes[string(answer.m.Payload)];ok {if time.Since(sent)>2*time.Second {t.Fatal("probe starved under bulk load")};delete(probes,string(answer.m.Payload));probeReplies++};continue}
   if answer.m.Kind==2 {t.Fatal("premature server KICK during bulk load")}
   f:=flows[answer.m.Connection];if f==nil{t.Fatal("cross-flow packet")}
   frame,e:=wire.DecodeFrame(answer.m.Payload);if e!=nil{t.Fatal(e)}
   controls,delivered:=f.reliable.Input(frame,time.Now());for _,control:=range controls {send(answer.m.Connection,control)}
   for _,data:=range delivered {switch data.DataKind {case 2:f.connected=true;case 0:f.received=append(f.received,data.Data...);case 4:default:t.Fatal("unexpected stream closure")}}
  }
 }
 if probeReplies==0 {t.Fatal("probe path was not exercised")}
 for _,f:=range flows {if !bytes.Equal(f.received,f.expected){t.Fatal("payload corrupted under load")}}
 actual,_:=db.GetUser(user.ID);if actual.Upload!=8*128*1024||actual.Download!=8*128*1024 {t.Fatalf("retransmission changed accounting: upload=%d download=%d",actual.Upload,actual.Download)}
}
