package wire
import("encoding/binary";"net";"testing")
func TestICMPv6EnvelopeAndPseudoChecksum(t *testing.T){
 source,destination:=net.ParseIP("2001:db8::1"),net.ParseIP("2001:db8::2")
 b:=Echo6(128,123,456,[]byte("payload"));binary.BigEndian.PutUint16(b[2:4],Checksum6(source,destination,b))
 if Checksum6(source,destination,b)!=0{t.Fatal("invalid pseudo-header checksum")}
 typ,id,seq,payload,err:=DecodeEcho6(b);if err!=nil||typ!=128||id!=123||seq!=456||string(payload)!="payload"{t.Fatal("ICMPv6 framing failed")}
 if _,_,_,_,err=DecodeEcho(b);err==nil{t.Fatal("IPv4 decoder accepted IPv6")}
 for _,bad:=range [][]byte{b[:7],append([]byte{128,1},b[2:]...),Echo6(135,1,1,nil)}{if _,_,_,_,err=DecodeEcho6(bad);err==nil{t.Fatal("invalid ICMPv6 accepted")}}
}
