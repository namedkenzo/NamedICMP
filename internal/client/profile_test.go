package client

import (
 "encoding/base64"
 "testing"
)
func TestProfileCredentialModes(t *testing.T){
 key:=base64.StdEncoding.EncodeToString(make([]byte,32))
 for _,link:=range []string{"pingtunnel://example.com?key=123&mode=proxy&lport=1080","pingtunnel://1.2.3.4?encrypt=aes256&encrypt_key="+key} {if _,e:=ParseProfile(link);e!=nil{t.Fatal(e)}}
 for _,link:=range []string{"pingtunnel://example.com?key=0","pingtunnel://example.com?key=12&encrypt=aes256&encrypt_key="+key,"pingtunnel://example.com?encrypt=aes256","pingtunnel://example.com?key=12&key=13","pingtunnel://example.com?key=12&lport=0","pingtunnel://[::1]?key=12","pingtunnel://example.com?key=12&mode=other","pingtunnel://example.com?key=12&encrypt_key=abc"}{if _,e:=ParseProfile(link);e==nil{t.Fatal("accepted invalid credential profile:",link)}}
}
func TestUDPHeaderRoundTrip(t *testing.T){for _,target:=range []string{"1.1.1.1:53","[2001:4860:4860::8888]:53","example.com:443"}{h,e:=udpHeader(target);if e!=nil{t.Fatal(e)};r:=&sliceReader{b:h[4:]};got,e:=readAddress(r,h[3]);if e!=nil || got!=target || len(r.b)!=0{t.Fatalf("roundtrip %s got %s %v",target,got,e)}}}
