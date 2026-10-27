package core

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"namedicmp/internal/store"
	"namedicmp/internal/wire"
)

type scriptedReceiveTransport struct {
	results chan transportResult
	done chan struct{}
	once sync.Once
}

func newScriptedReceiver() *scriptedReceiveTransport {
	return &scriptedReceiveTransport{results:make(chan transportResult,4),done:make(chan struct{})}
}
func (t *scriptedReceiveTransport) Read() (Packet,error) {
	select {
	case result:=<-t.results:
		return result.packet,result.err
	case <-t.done:
		return Packet{},net.ErrClosed
	}
}
func (t *scriptedReceiveTransport) Write(Packet,wire.Message) error {return nil}
func (t *scriptedReceiveTransport) Close() error {t.once.Do(func(){close(t.done)});return nil}

func TestTransientReceiveErrorClassification(t *testing.T) {
	for _,err:=range []error{syscall.EINTR,syscall.EAGAIN,syscall.EWOULDBLOCK,syscall.ENOBUFS,syscall.ENOMEM} {
		wrapped:=&net.OpError{Op:"read",Net:"ip4:icmp",Err:&os.SyscallError{Syscall:"recvfrom",Err:err}}
		if !transientReceiveError(wrapped) {t.Fatalf("temporary kernel receive error treated as fatal: %v",wrapped)}
	}
	for _,err:=range []error{nil,net.ErrClosed,io.EOF,syscall.EINVAL,syscall.EPERM,errors.New("no buffer space available")} {
		if transientReceiveError(err) {t.Fatalf("permanent/unknown error treated as transient: %v",err)}
	}
}

func TestServeRecoversAfterTransientReceiveError(t *testing.T) {
	db,err:=store.Open(filepath.Join(t.TempDir(),"state.json"))
	if err!=nil {t.Fatal(err)}
	defer db.Close()
	transport:=newScriptedReceiver()
	defer transport.Close()
	transport.results<-transportResult{err:&os.SyscallError{Syscall:"recvfrom",Err:syscall.ENOBUFS}}
	transport.results<-transportResult{packet:Packet{Source:"192.0.2.1"}}
	c:=&Core{Store:db,transport:transport}
	ctx,cancel:=context.WithCancel(context.Background())
	defer cancel()
	finished:=make(chan error,1)
	go func(){finished<-c.Serve(ctx)}()
	deadline:=time.NewTimer(time.Second)
	defer deadline.Stop()
	tick:=time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for c.received.Load()==0 {
		select {
		case err:=<-finished:t.Fatalf("service exited before receiving next packet: %v",err)
		case <-deadline.C:t.Fatal("receive did not resume after transient pressure")
		case <-tick.C:
		}
	}
	cancel()
	select {
	case err:=<-finished:if err!=nil {t.Fatal(err)}
	case <-time.After(time.Second):t.Fatal("cancelled receive loop did not exit")
	}
}

func TestServePreservesPermanentReceiveError(t *testing.T) {
	transport:=newScriptedReceiver()
	defer transport.Close()
	failure:=errors.New("permanent raw socket failure")
	transport.results<-transportResult{err:failure}
	c:=&Core{transport:transport}
	err:=c.Serve(context.Background())
	if !errors.Is(err,failure)||!strings.Contains(err.Error(),"ICMP receive failed") {t.Fatalf("permanent error lost context: %v",err)}
}

func TestReceiveBackoffBoundAndCancellation(t *testing.T) {
	done:=make(chan struct{})
	delay:=time.Duration(0)
	if !waitReceiveRetry(done,&delay)||delay!=5*time.Millisecond {t.Fatal("missing initial bounded delay")}
	close(done)
	// Even an already saturated backoff stops on cancellation without waiting
	// for the timer. The delay can never grow beyond half a second.
	delay=500*time.Millisecond
	start:=time.Now()
	if waitReceiveRetry(done,&delay)||delay!=500*time.Millisecond {t.Fatal("unbounded/uncancellable backoff")}
	if time.Since(start)>100*time.Millisecond {t.Fatal("backoff delayed cancellation")}
}

func TestReceiveErrorNoticesAreRateLimited(t *testing.T) {
	var last time.Time
	if !receiveErrorNotice(&last) {t.Fatal("first receive notice suppressed")}
	if receiveErrorNotice(&last) {t.Fatal("receive error flooded logs")}
	last=time.Now().Add(-31*time.Second)
	if !receiveErrorNotice(&last) {t.Fatal("persistent receive error disappeared from logs")}
}

func TestDualReaderWorkerSurvivesTransientReceiveError(t *testing.T) {
	source:=newScriptedReceiver()
	dual:=&DualTransport{in:make(chan transportResult,1),done:make(chan struct{})}
	defer func(){source.Close();dual.Close()}()
	source.results<-transportResult{err:syscall.ENOBUFS}
	source.results<-transportResult{packet:Packet{Source:"192.0.2.20",Family:6}}
	dual.startReader("ICMPv6",source.Read)
	result:=make(chan transportResult,1)
	go func(){p,err:=dual.Read();result<-transportResult{p,err}}()
	select {
	case got:=<-result:
		if got.err!=nil||got.packet.Source!="192.0.2.20" {t.Fatalf("worker exited on recoverable receive error: %+v",got)}
		if dual.ReceiveRetries()!=1 {t.Fatal("recoverable receive error was not accounted")}
	case <-time.After(time.Second):t.Fatal("dual reader failed to resume")
	}
}

func TestDualReaderPermanentFailureHasFamilyContext(t *testing.T) {
	source:=newScriptedReceiver()
	dual:=&DualTransport{in:make(chan transportResult,1),done:make(chan struct{})}
	defer func(){source.Close();dual.Close()}()
	source.results<-transportResult{err:syscall.EPERM}
	dual.startReader("ICMPv4",source.Read)
	_,err:=dual.Read()
	if !errors.Is(err,syscall.EPERM)||!strings.Contains(err.Error(),"ICMPv4 receive failed") {t.Fatalf("permanent family failure hidden: %v",err)}
}

func TestDualReaderCancelsDuringRetry(t *testing.T) {
	dual:=&DualTransport{in:make(chan transportResult,1),done:make(chan struct{})}
	entered:=make(chan struct{})
	var once sync.Once
	dual.startReader("ICMPv4",func()(Packet,error){once.Do(func(){close(entered)});return Packet{},syscall.ENOBUFS})
	<-entered
	finished:=make(chan struct{})
	go func(){dual.Close();close(finished)}()
	select {
	case <-finished:
	case <-time.After(time.Second):t.Fatal("dual retry did not stop on close")
	}
}
