package core
import("testing";"namedicmp/internal/wire")
func TestUDPBypassesBulkWithoutStarvingTCP(t *testing.T){
 c:=&Core{udpOutQueue:make(chan responsePacket,64),dataOut:make(chan responsePacket,2048)}
 for i:=0;i<64;i++{c.udpOutQueue<-responsePacket{message:wire.Message{Target:"udp"}}}
 for i:=0;i<2048;i++{c.dataOut<-responsePacket{message:wire.Message{Target:"tcp"}}}
 n:=0;for i:=0;i<20;i++{p,ok:=c.nextData(&n);if !ok{t.Fatal("missing data")};want:="udp";if i%5==4{want="tcp"};if p.message.Target!=want{t.Fatalf("packet %d: %s not %s",i,p.message.Target,want)}}
}
func TestFullUDPQueueKeepsNewerPackets(t *testing.T){
 c:=&Core{udpOutQueue:make(chan responsePacket,64),dataOut:make(chan responsePacket,1),controlOut:make(chan responsePacket,1),stopWrite:make(chan struct{})}
 for i:=0;i<100;i++{c.reply(Packet{},wire.Message{Payload:[]byte{byte(i)}})}
 if len(c.udpOutQueue)!=64||c.writeQueueDrops.Load()!=36{t.Fatal("queue not bounded/accounted")}
 for i:=36;i<100;i++{p:=<-c.udpOutQueue;if p.message.Payload[0]!=byte(i){t.Fatal("old UDP retained")}}
}
