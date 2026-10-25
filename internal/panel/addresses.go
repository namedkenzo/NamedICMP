package panel

import (
 "net"
 "net/netip"
 "net/http"
 "context"
 "io"
 "os"
 "strings"
 "time"
 "sort"
 "namedicmp/internal/store"
)
// Interface detection does not contact a third party or guess a NAT public IP.
// NAT installations can use the explicit IP/domain field in Settings.
func detectedAddresses()map[string][]string{
 result:=map[string][]string{"ipv4":{},"ipv6":{}}
 interfaces,_:=net.Interfaces();seen:=map[string]bool{}
 for _,iface:=range interfaces {if iface.Flags&net.FlagUp==0||iface.Flags&net.FlagLoopback!=0{continue};addresses,_:=iface.Addrs();for _,a:=range addresses{
  prefix,err:=netip.ParsePrefix(a.String());if err!=nil{continue};ip:=prefix.Addr().Unmap()
  if !publicAddress(ip)||seen[ip.String()]{continue}
  family:="ipv6";if ip.Is4(){family="ipv4"};seen[ip.String()]=true;result[family]=append(result[family],ip.String())
 }}
 for _,ips:=range result{sort.Strings(ips)};return result
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
func publicAddress(ip netip.Addr)bool{if !ip.IsGlobalUnicast()||ip.IsPrivate()||ip.IsLinkLocalUnicast(){return false};for _,prefix:=range nonPublic{if prefix.Contains(ip){return false}};return true}
