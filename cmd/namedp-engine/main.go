// namedp-engine accepts secrets on stdin, never in its command line.
package main

import (
 "context"
 "encoding/json"
 "fmt"
 "io"
 "os"
 "sync"
 "time"
 "namedicmp/internal/client"
)
const version = "0.1.0-alpha"

func main(){if len(os.Args)==2 && os.Args[1]=="--version"{fmt.Println(version);return};if err:=run();err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}}
func run()error{
 var input struct{Link string `json:"link"`}
 dec:=json.NewDecoder(io.LimitReader(os.Stdin,16384));if err:=dec.Decode(&input);err!=nil{return err}
 p,err:=client.ParseProfile(input.Link);if err!=nil{return err}
 ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 // The GUI retains stdin for this process's lifetime. Closing the GUI's pipe
 // terminates the engine even if its parent was killed.
 go func(){io.Copy(io.Discard,os.Stdin);cancel()}()
 t,err:=client.Open(ctx,p);if err!=nil{return err};defer t.Close()
 probe,stop:=context.WithTimeout(ctx,8*time.Second);err=t.Probe(probe);stop();if err!=nil{return err}
 var mu sync.Mutex;emit:=func(v any){mu.Lock();defer mu.Unlock();json.NewEncoder(os.Stdout).Encode(v)}
 go func(){tick:=time.NewTicker(time.Second);defer tick.Stop();for{select{case <-ctx.Done():return;case <-tick.C:emit(map[string]any{"event":"stats","upload":t.Upload.Load(),"download":t.Download.Load()})}}}()
 go func(){tick:=time.NewTicker(5*time.Second);defer tick.Stop();for{select{case <-ctx.Done():return;case <-t.Done():return;case <-tick.C:
  probe,done:=context.WithTimeout(ctx,8*time.Second);err:=t.Probe(probe);done();if err!=nil{fmt.Fprintln(os.Stderr,"ICMP connection lost; reconnect after checking the server and network.");t.Close();return}
 }}}()
 err=t.ServeSOCKS(ctx,p.Port,func(){emit(map[string]any{"event":"ready","server_ip":t.ServerIP(),"port":p.Port})})
 if ctx.Err()!=nil{return nil};return err
}
