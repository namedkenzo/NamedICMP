package panel

import (
 "bufio"
 "encoding/hex"
 "net"
 "net/netip"
 "net/http"
 "context"
 "io"
 "os"
 "strings"
 "time"
 "sort"
 "strconv"
 "namedicmp/internal/store"
)
// Interface detection does not contact a third party or guess a NAT public IP.
// NAT installations can use the explicit IP/domain field in Settings.
func detectedAddresses()map[string][]string{
 var proc io.Reader
 if file,err:=os.Open("/proc/net/if_inet6");err==nil{defer file.Close();proc=file}
 return detectAddressSources(net.Interfaces,func(iface net.Interface)([]net.Addr,error){return iface.Addrs()},proc,interfaceReady)
}

// Keep local discovery independent of external IP lookup. In particular,
// Linux interface enumeration uses AF_NETLINK, which older service sandboxes
// blocked. The proc inventory is a read-only fallback and also supplies IPv6
// readiness flags that net.Interface.Addrs does not expose.
func detectAddressSources(interfaces func()([]net.Interface,error),addrs func(net.Interface)([]net.Addr,error),proc io.Reader,ready func(string)bool)map[string][]string{
 result:=map[string][]string{"ipv4":{},"ipv6":{}}
 inventory,err:=interfaces();seen:=map[string]bool{};linkReady:=map[string]bool{}
 if err==nil{for _,iface:=range inventory{linkReady[iface.Name]=iface.Flags&net.FlagUp!=0&&iface.Flags&net.FlagLoopback==0}}
 if err==nil{for _,iface:=range inventory {if iface.Flags&net.FlagUp==0||iface.Flags&net.FlagLoopback!=0{continue};addresses,err:=addrs(iface);if err!=nil{continue};for _,a:=range addresses{
  prefix,err:=netip.ParsePrefix(a.String());if err!=nil{continue};ip:=prefix.Addr().Unmap()
  if !publicAddress(ip)||seen[ip.String()]{continue}
  family:="ipv6";if ip.Is4(){family="ipv4"};seen[ip.String()]=true;result[family]=append(result[family],ip.String())
 }}}
 procReady:=func(name string)bool{if up,known:=linkReady[name];known{return up};return ready(name)}
 if proc!=nil{if addresses,err:=procIPv6Addresses(proc,procReady);err==nil{result["ipv6"]=addresses}}
 for _,ips:=range result{sort.Strings(ips)};return result
}

func interfaceReady(name string)bool{
 if name==""||name=="."||name==".."||strings.ContainsAny(name,"/\\"){return false}
 value,err:=os.ReadFile("/sys/class/net/"+name+"/flags");if err!=nil{return false}
 flags,err:=strconv.ParseUint(strings.TrimSpace(string(value)),0,32)
 return err==nil&&flags&1!=0&&flags&8==0 // IFF_UP, not IFF_LOOPBACK.
}

func procIPv6Addresses(source io.Reader,ready func(string)bool)([]string,error){
 result:=[]string{};seen:=map[string]bool{};scanner:=bufio.NewScanner(source)
 for scanner.Scan(){
  fields:=strings.Fields(scanner.Text());if len(fields)!=6||len(fields[0])!=32{continue}
  raw,err:=hex.DecodeString(fields[0]);if err!=nil{continue}
  scope,err:=strconv.ParseUint(fields[3],16,32);if err!=nil||scope!=0{continue}
  flags,err:=strconv.ParseUint(fields[4],16,32)
  // IFA_F_DADFAILED, IFA_F_DEPRECATED and IFA_F_TENTATIVE cannot be
  // exported as a ready endpoint; a global scope alone is not enough.
  if err!=nil||flags&(0x08|0x20|0x40)!=0||!ready(fields[5]){continue}
  ip,ok:=netip.AddrFromSlice(raw);if !ok||!publicAddress(ip)||seen[ip.String()]{continue}
  seen[ip.String()]=true;result=append(result,ip.String())
 }
 return result,scanner.Err()
}
func(p *Panel)connection()store.ConnectionSettings {
 if value,ok:=p.Store.Connection();ok{return value}
 family:=p.Config.EndpointFamily;if family==""{family="ipv4"}
 return store.ConnectionSettings{Family:family,Host:p.Config.PublicHost,ReaderMode:"raw_ip"}
}
func(p *Panel)connectionInfo()map[string]any{
 value:=p.connection();addresses:=p.networkAddresses();host:=value.Host
 if host=="" && len(addresses[value.Family])>0{host=addresses[value.Family][0]}
 families:=[]int{};reader:="off";if p.Core!=nil{families=p.Core.Families();reader=p.Core.ReaderMode()}
 warning:="";if p.Core!=nil{warning=p.Core.ReaderWarning()}
 return map[string]any{"family":value.Family,"host":value.Host,"effective_host":host,"reader_mode":value.ReaderMode,"active_reader_mode":reader,"reader_warning":warning,"restart_required":p.Core!=nil&&reader!=value.ReaderMode,"addresses":addresses,"listen_families":families}
}
func(p *Panel)enableAddressLookup(){p.mu.Lock();p.addrLookup=os.Getenv("NAMEDICMP_PUBLIC_IP_LOOKUP")!="0";p.mu.Unlock()}
func(p *Panel)networkAddresses()map[string][]string{
 local:=detectedAddresses()
 p.mu.Lock();defer p.mu.Unlock()
 for family,addresses:=range p.addrCache{if len(local[family])==0{local[family]=append([]string(nil),addresses...)}}
 if p.addrLookup&&!p.addrBusy&&(p.addrAt.IsZero()||time.Since(p.addrAt)>5*time.Minute){
  p.addrBusy=true;go func(){
   result:=detectedAddresses()
   for _,family:=range []string{"ipv4","ipv6"}{if len(result[family])==0{if ip:=publicIP(family);ip!=""{result[family]=[]string{ip}}}}
   p.mu.Lock();p.addrCache=result;p.addrAt=time.Now();p.addrBusy=false;p.mu.Unlock()
  }()
 }
 return local
}
// Only a public-IP echo endpoint is contacted, with no credentials or user
// traffic. Proxy-configured installs skip this to avoid exporting a proxy IP.
func publicIP(family string)string{
 endpoint,network:="https://api.ipify.org","tcp4";if family=="ipv6"{endpoint,network="https://api6.ipify.org","tcp6"}
 ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);defer cancel()
 request,_:=http.NewRequestWithContext(ctx,"GET",endpoint,nil)
 proxy,err:=http.ProxyFromEnvironment(request);if err!=nil||proxy!=nil{return ""}
 transport:=&http.Transport{DialContext:func(ctx context.Context,_,address string)(net.Conn,error){return (&net.Dialer{}).DialContext(ctx,network,address)}};defer transport.CloseIdleConnections()
 client:=&http.Client{Transport:transport,Timeout:2*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
 response,err:=client.Do(request);if err!=nil{return ""};defer response.Body.Close();if response.StatusCode!=200{return ""}
 body,err:=io.ReadAll(io.LimitReader(response.Body,65));if err!=nil||len(body)>64{return ""}
 ip,err:=netip.ParseAddr(strings.TrimSpace(string(body)));if err!=nil||!publicAddress(ip)||ip.Is4In6()||(ip.Is4()!=(family=="ipv4")){return ""};return ip.String()
}
var nonPublic=[]netip.Prefix{netip.MustParsePrefix("0.0.0.0/8"),netip.MustParsePrefix("100.64.0.0/10"),netip.MustParsePrefix("192.0.0.0/24"),netip.MustParsePrefix("192.0.2.0/24"),netip.MustParsePrefix("198.18.0.0/15"),netip.MustParsePrefix("198.51.100.0/24"),netip.MustParsePrefix("203.0.113.0/24"),netip.MustParsePrefix("240.0.0.0/4"),netip.MustParsePrefix("2001:db8::/32")}
func publicAddress(ip netip.Addr)bool{if ip.Zone()!=""||ip.Is4In6()||!ip.IsGlobalUnicast()||ip.IsPrivate()||ip.IsLinkLocalUnicast(){return false};for _,prefix:=range nonPublic{if prefix.Contains(ip){return false}};return true}
