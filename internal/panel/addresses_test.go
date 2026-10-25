package panel

import (
 "errors"
 "net"
 "reflect"
 "strings"
 "testing"
)

func TestIPv6DetectionSurvivesBlockedNetlink(t *testing.T){
 proc:="26064700470000000000000000001111 02 40 00 80 eth0\n"+
  "fe800000000000000000000000000001 02 40 20 80 eth0\n"
 got:=detectAddressSources(func()([]net.Interface,error){return nil,errors.New("socket: address family not supported")},func(net.Interface)([]net.Addr,error){t.Fatal("address enumeration called after failure");return nil,nil},strings.NewReader(proc),func(name string)bool{return name=="eth0"})
 if !reflect.DeepEqual(got["ipv6"],[]string{"2606:4700:4700::1111"}){t.Fatalf("public IPv6 lost when netlink is restricted: %v",got)}
 if len(got["ipv4"])!=0{t.Fatal("invented an IPv4 address")}
}

func TestProcIPv6RequiresReadyPublicAddressAndUpInterface(t *testing.T){
 lines:=[]string{
  "26064700470000000000000000001111 02 40 00 80 eth0", // valid
  "26064700470000000000000000001111 02 40 00 80 eth0", // duplicate
  "26064700470000000000000000001112 02 40 00 40 eth0", // tentative
  "26064700470000000000000000001113 02 40 00 08 eth0", // DAD failure
  "26064700470000000000000000001114 02 40 00 20 eth0", // deprecated
  "26064700470000000000000000001115 02 40 00 80 down0",
  "fe800000000000000000000000000001 02 40 20 80 eth0",
  "fd000000000000000000000000000001 02 40 00 80 eth0",
  "20010db8000000000000000000000001 02 40 00 80 eth0",
  "00000000000000000000000000000001 01 80 10 80 lo",
  "zzzz4700470000000000000000001111 02 40 00 80 eth0",
  "malformed",
 }
 got,err:=procIPv6Addresses(strings.NewReader(strings.Join(lines,"\n")),func(name string)bool{return name=="eth0"})
 if err!=nil||!reflect.DeepEqual(got,[]string{"2606:4700:4700::1111"}){t.Fatalf("invalid detection candidates: %v %v",got,err)}
}

func TestProcFlagsOverrideIncompleteInterfaceIPv6Inventory(t *testing.T){
 interfaces:=func()([]net.Interface,error){return []net.Interface{{Name:"eth0",Flags:net.FlagUp}},nil}
 addrs:=func(net.Interface)([]net.Addr,error){return []net.Addr{&net.IPNet{IP:net.ParseIP("2606:4700:4700::1111"),Mask:net.CIDRMask(64,128)},&net.IPNet{IP:net.ParseIP("8.8.8.8"),Mask:net.CIDRMask(24,32)}},nil}
 got:=detectAddressSources(interfaces,addrs,strings.NewReader("26064700470000000000000000001111 02 40 00 40 eth0\n"),func(string)bool{return true})
 if len(got["ipv6"])!=0{t.Fatal("tentative IPv6 exported from incomplete interface inventory")}
 if !reflect.DeepEqual(got["ipv4"],[]string{"8.8.8.8"}){t.Fatal("IPv4 inventory was changed",got)}
}

func TestIPv6InterfaceSourceWorksWithoutLinuxProc(t *testing.T){
 got:=detectAddressSources(func()([]net.Interface,error){return []net.Interface{{Name:"eth0",Flags:net.FlagUp}},nil},func(net.Interface)([]net.Addr,error){return []net.Addr{&net.IPNet{IP:net.ParseIP("2606:4700:4700::1111"),Mask:net.CIDRMask(64,128)}},nil},nil,func(string)bool{return false})
 if len(got["ipv6"])!=1{t.Fatal("non-proc detection regressed",got)}
}

func TestNetlinkInterfaceReadinessDoesNotDependOnHostSysfs(t *testing.T){
 got:=detectAddressSources(func()([]net.Interface,error){return []net.Interface{{Name:"namespace0",Flags:net.FlagUp}},nil},func(net.Interface)([]net.Addr,error){return nil,nil},strings.NewReader("26064700470000000000000000001111 02 40 00 80 namespace0\n"),func(string)bool{t.Fatal("host sysfs consulted for known namespace interface");return false})
 if !reflect.DeepEqual(got["ipv6"],[]string{"2606:4700:4700::1111"}){t.Fatal("namespace IPv6 discarded because sysfs was unavailable",got)}
}
