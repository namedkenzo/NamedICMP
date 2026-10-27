package panel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"namedicmp/internal/store"
)

func monitorPanel(t *testing.T) *Panel {
	t.Helper()
	s,err:=store.Open(filepath.Join(t.TempDir(),"state.json"));if err!=nil {t.Fatal(err)}
	t.Cleanup(func(){s.Close()})
	conf:=store.DefaultConfig();conf.StatePath=filepath.Join(t.TempDir(),"state.json")
	p:=New(s,nil,conf)
	p.sessions["test-session"]=Session{CSRF:"test-csrf",Expires:time.Now().Add(time.Hour)}
	return p
}

func TestLiveOriginAuthRevocation(t *testing.T) {
	p:=monitorPanel(t);srv:=httptest.NewServer(p.Handler());defer srv.Close()
	endpoint:="ws"+strings.TrimPrefix(srv.URL,"http")+"/api/live"
	for _,test:=range []struct{cookie,origin string;code int}{{"",srv.URL,401},{"namedicmp_session=test-session","",403},{"namedicmp_session=test-session","https://evil.example",403}} {
		headers:=http.Header{};headers.Set("Cookie",test.cookie);headers.Set("Origin",test.origin)
		conn,response,err:=websocket.DefaultDialer.Dial(endpoint,headers)
		if conn!=nil {conn.Close();t.Fatal("unauthorized WebSocket accepted")}
		if err==nil||response==nil||response.StatusCode!=test.code {t.Fatalf("expected %d, response=%v err=%v",test.code,response,err)}
		response.Body.Close()
	}
	conn,_,err:=websocket.DefaultDialer.Dial(endpoint,http.Header{"Cookie":{"namedicmp_session=test-session"},"Origin":{srv.URL}})
	if err!=nil {t.Fatal(err)};defer conn.Close()
	var frame struct{Type string `json:"type"`;Data map[string]any `json:"data"`;History []trafficPoint `json:"history"`}
	conn.SetReadDeadline(time.Now().Add(3*time.Second))
	if err=conn.ReadJSON(&frame);err!=nil {t.Fatal(err)}
	if frame.Type!="snapshot"||len(frame.History)!=1||frame.Data["core_enabled"]!=false {t.Fatalf("bad snapshot %+v",frame)}
	p.sampleLive(time.Now())
	if err=conn.ReadJSON(&frame);err!=nil||frame.Type!="sample" {t.Fatalf("no live update: %v %+v",err,frame)}
	p.ClearSessions()
	conn.SetReadDeadline(time.Now().Add(2500*time.Millisecond))
	_,_,err=conn.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err,&closeErr) {t.Fatalf("revoked session was not closed: %v",err)}
}

func TestLiveBoundedHistoryAndSlowSubscriber(t *testing.T) {
	p:=monitorPanel(t)
	channels:=[]chan []byte{}
	for i:=0;i<liveClientLimit;i++ {ch,_,ok:=p.Live.subscribe();if !ok {t.Fatal("early limit")};channels=append(channels,ch)}
	if _,_,ok:=p.Live.subscribe();ok {t.Fatal("unbounded clients")}
	for i:=0;i<historyLimit+3;i++ {p.sampleLive(time.Now().Add(time.Duration(i)*time.Second))}
	if len(p.Live.history)!=historyLimit {t.Fatal("unbounded history")}
	for _,ch:=range channels {if len(ch)!=1 {t.Fatal("slow subscriber queue grew")};var frame map[string]any;if err:=json.Unmarshal(<-ch,&frame);err!=nil {t.Fatal(err)};p.Live.unsubscribe(ch)}
	if len(p.Live.clients)!=0 {t.Fatal("subscriber leak")}
	for _,test:=range []struct{now,before uint64;seconds,want float64}{{200,100,2,50},{20,100,1,0},{200,100,0,0}} {if got:=counterRate(test.now,test.before,test.seconds);got!=test.want {t.Fatalf("rate %v",got)}}
}

func TestLiveShutdown(t *testing.T) {
	p:=monitorPanel(t)
	ctx,cancel:=context.WithCancel(context.Background());done:=make(chan struct{})
	go func(){p.RunStats(ctx);close(done)}()
	cancel()
	select{case <-done:case <-time.After(time.Second):t.Fatal("monitor did not stop")}
	if _,_,ok:=p.Live.subscribe();ok {t.Fatal("accepted subscriber after shutdown")}
}

func TestProbeWindowAndJitter(t *testing.T) {
	p:=newPathProbe("ipv4","1.1.1.1",true)
	if p.snapshot().Loss!=nil {t.Fatal("unmeasured loss reported as zero")}
	for _,ms:=range []int{10,20,0,40,60} {p.record(time.Now(),echoResult{sent:true,rtt:time.Duration(ms)*time.Millisecond})}
	s:=p.snapshot()
	if s.Sent!=5||s.Received!=4||*s.Loss!=20||*s.Jitter!=15||*s.Average!=32.5||*s.Minimum!=10||*s.Maximum!=60 {t.Fatalf("wrong probe stats %+v",s)}
	p.record(time.Now(),echoResult{err:errors.New("permission denied")})
	s=p.snapshot();if s.Sent!=5||s.TotalSent!=5||s.RTT!=nil||s.Status!="unavailable" {t.Fatal("failed send counted as packet loss")}
	for i:=0;i<40;i++ {p.record(time.Now(),echoResult{sent:true,rtt:time.Millisecond})}
	if s=p.snapshot();s.Sent!=30||s.TotalSent!=45||*s.Loss!=0 {t.Fatal("window unbounded")}
}

func TestKernelParsers(t *testing.T) {
	v4:=parseICMPv4([]byte("Ip: InReceives\nIp: 2\nIcmp: InMsgs InErrors OutMsgs\nIcmp: 100 3 50\n"))
	if v4["InMsgs"]!=100||v4["InErrors"]!=3 {t.Fatal(v4)}
	if parseICMPv4([]byte("Icmp: InMsgs OutMsgs\nIcmp: 2\n"))!=nil {t.Fatal("malformed proc counters accepted")}
	v6:=parseICMPv6([]byte("Ip6InReceives 100\nIcmp6InMsgs 9\nIcmp6InErrors 1\n"))
	if len(v6)!=2||v6["InMsgs"]!=9 {t.Fatal(v6)}
	if parseICMPv6(nil)!=nil {t.Fatal("missing counters should be unavailable")}
}

// CI also runs this compiled test as root, exercising real Linux checksums,
// family-specific filters and reply matching without contacting the Internet.
func TestEchoLoopback(t *testing.T) {
	for _,test:=range []struct{family,target string}{{"ipv4","127.0.0.1"},{"ipv6","::1"}} {
		t.Run(test.family,func(t *testing.T){
			ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel()
			result:=echoOnce(ctx,test.family,test.target)
			if result.err!=nil {if os.Geteuid()==0 {t.Fatal(result.err)};t.Skipf("raw socket unavailable: %v",result.err)}
			if !result.sent||result.rtt<=0 {t.Fatal("no matching loopback echo reply")}
		})
	}
}
