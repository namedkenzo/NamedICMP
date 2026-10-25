package store

import (
 "errors"
 "net/netip"
 "strings"
)
type ConnectionSettings struct { Family string `json:"family"`; Host string `json:"host"`; ReaderMode string `json:"reader_mode"` }
func ValidateConnection(v ConnectionSettings)(ConnectionSettings,error){
 if v.Family!="ipv4"&&v.Family!="ipv6" {return v,errors.New("choose IPv4 or IPv6")}
 if v.ReaderMode==""{v.ReaderMode="raw_ip"}
 if v.ReaderMode!="raw_ip"&&v.ReaderMode!="raw_packet"{return v,errors.New("invalid packet reader; NEW is a firewall state, not a read mode")}
 v.Host=strings.TrimSpace(v.Host)
 if strings.HasPrefix(v.Host,"[")&&strings.HasSuffix(v.Host,"]"){v.Host=v.Host[1:len(v.Host)-1]}
 if v.Host==""{return v,nil}
 if addr,err:=netip.ParseAddr(v.Host);err==nil {
  if addr.Zone()!=""||addr.Is4In6()||addr.IsUnspecified()||addr.IsMulticast()||addr.IsLinkLocalUnicast(){return v,errors.New("use an unscoped unicast connection address")}
  if (v.Family=="ipv4")!=addr.Is4(){return v,errors.New("address does not match selected family")}
  v.Host=addr.String();return v,nil
 }
 if len(v.Host)>253 || strings.ContainsAny(v.Host,":/@?#%\\ \t\r\n") {return v,errors.New("enter only an IP address or domain, without scheme or port")}
 v.Host=strings.ToLower(strings.TrimSuffix(v.Host,"."))
 for _,label:=range strings.Split(v.Host,"."){
  if len(label)==0||len(label)>63||label[0]=='-'||label[len(label)-1]=='-' {return v,errors.New("invalid domain")}
  for _,r:=range label {if !(r>='a'&&r<='z'||r>='0'&&r<='9'||r=='-'){return v,errors.New("use an ASCII domain or its punycode form")}}
 }
 return v,nil
}
func(s *Store)Connection()(ConnectionSettings,bool){s.mu.Lock();defer s.mu.Unlock();if s.d.Connection==nil{return ConnectionSettings{},false};return *s.d.Connection,true}
func(s *Store)SetConnection(v ConnectionSettings)error{
 v,err:=ValidateConnection(v);if err!=nil{return err};s.mu.Lock();defer s.mu.Unlock()
 if s.fault!=nil{return s.fault};old:=s.d.Connection;s.d.Connection=&v
 if err:=s.save();err!=nil{s.d.Connection=old;s.fault=err;return err};return nil
}
