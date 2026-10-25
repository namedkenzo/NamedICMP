package wire

import (
 "encoding/binary"
 "net"
)

func Checksum6(source,destination net.IP,message []byte)uint16{
 b:=make([]byte,40+len(message));copy(b[:16],source.To16());copy(b[16:32],destination.To16());binary.BigEndian.PutUint32(b[32:36],uint32(len(message)));b[39]=58;copy(b[40:],message);return Checksum(b)
}

// ICMPv6 uses a pseudo-header checksum. Raw ICMPv6 and ping sockets calculate
// it on send and validate it on receive in the OS; an IPv4 checksum is invalid.
func Echo6(typ byte,id,seq uint16,payload []byte)[]byte {
 b:=make([]byte,8+len(payload));b[0]=typ
 binary.BigEndian.PutUint16(b[4:6],id);binary.BigEndian.PutUint16(b[6:8],seq);copy(b[8:],payload)
 return b
}
func DecodeEcho6(b []byte)(byte,uint16,uint16,[]byte,error){
 if len(b)>=40 && b[0]>>4==6 {
  // IPConn normally removes the IP header. Replay inputs with extension or
  // fragment headers are deliberately rejected instead of guessed.
  if b[6]!=58 || int(binary.BigEndian.Uint16(b[4:6]))!=len(b)-40{return 0,0,0,nil,ErrMalformed}
  b=b[40:]
 }
 if len(b)<8||b[1]!=0||(b[0]!=128&&b[0]!=129){return 0,0,0,nil,ErrMalformed}
 return b[0],binary.BigEndian.Uint16(b[4:6]),binary.BigEndian.Uint16(b[6:8]),b[8:],nil
}
