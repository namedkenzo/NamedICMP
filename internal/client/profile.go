// Package client implements the namedP client without an upstream executable.
package client

import (
 "errors"
 "net"
 "net/url"
 "strconv"
 "strings"
 "encoding/base64"
 "namedicmp/internal/wire"
)

type Profile struct { Host string; Port int; Mode string; Credential wire.Credential }

func ParseProfile(link string) (Profile,error) {
 p:=Profile{Port:1080,Mode:"vpn"}
 u,err:=url.Parse(strings.TrimSpace(link))
 if err!=nil || u.Scheme!="pingtunnel" || u.Hostname()=="" || u.Port()!="" || u.User!=nil || u.Path!="" || u.Fragment!="" {return p,errors.New("invalid pingtunnel link")}
 if net.ParseIP(u.Hostname())!=nil && net.ParseIP(u.Hostname()).To4()==nil {return p,errors.New("the tunnel server must support IPv4")}
 p.Host=u.Hostname();q,err:=url.ParseQuery(u.RawQuery);if err!=nil{return p,errors.New("invalid link encoding")}
 for k,v:=range q {if len(v)!=1{return p,errors.New("duplicate link parameter")};switch k {case "lport","mode","key","encrypt","encrypt_key":default:return p,errors.New("unsupported link parameter: "+k)}}
 if q.Has("lport") {p.Port,err=strconv.Atoi(q.Get("lport"));if err!=nil || p.Port<1 || p.Port>65535{return p,errors.New("invalid local port")}}
 if q.Has("mode"){p.Mode=q.Get("mode")};if p.Mode!="vpn" && p.Mode!="proxy"{return p,errors.New("invalid connection mode")}
 mode,err:=wire.NormalizeEncryption(q.Get("encrypt"));if err!=nil{return p,err};p.Credential.Mode=mode
 if mode=="none" {n,e:=strconv.ParseInt(q.Get("key"),10,32);if e!=nil || n<=0 || q.Has("encrypt_key"){return p,errors.New("numeric mode requires one positive numeric key")};p.Credential.Key=int32(n)
 } else {if q.Has("key"){return p,errors.New("choose encryption OR a numeric key")};secret:=q.Get("encrypt_key");b,e:=base64.StdEncoding.DecodeString(secret);if e!=nil || len(b)!=wire.EncryptionKeySize(mode){return p,errors.New("incomplete or invalid encryption key; copy the whole link")};p.Credential.Key=1;p.Credential.Secret=secret}
 return p,nil
}
