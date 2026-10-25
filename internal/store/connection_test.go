package store

import("path/filepath";"testing")
func TestConnectionSettingsValidationAndPersistence(t *testing.T){
 for _,v:=range []ConnectionSettings{{"ipv4","203.0.113.4","raw_ip"},{"ipv6","[2001:db8::4]","raw_packet"},{"ipv6","vpn.example.com","raw_ip"},{"ipv4","","raw_ip"}}{if _,err:=ValidateConnection(v);err!=nil{t.Fatal(err)}}
 for _,v:=range []ConnectionSettings{{"ipv4","::1","raw_ip"},{"ipv6","192.0.2.1","raw_ip"},{"ipv6","fe80::1%eth0","raw_ip"},{"ipv4","https://host.example","raw_ip"},{"ipv4","bad;command","raw_ip"},{"ipv6","::ffff:192.0.2.1","raw_ip"},{"ipv4","","new"}}{if _,err:=ValidateConnection(v);err==nil{t.Fatalf("accepted invalid setting %#v",v)}}
 path:=filepath.Join(t.TempDir(),"state.json");s,err:=Open(path);if err!=nil{t.Fatal(err)}
 user,err:=s.PutUser(User{Name:"kept",Enabled:true});if err!=nil{t.Fatal(err)}
 if err=s.SetConnection(ConnectionSettings{"ipv6","vpn.example.com","raw_packet"});err!=nil{t.Fatal(err)}
 if err=s.Close();err!=nil{t.Fatal(err)};s,err=Open(path);if err!=nil{t.Fatal(err)};defer s.Close()
 settings,ok:=s.Connection();if !ok||settings.Family!="ipv6"||settings.ReaderMode!="raw_packet"{t.Fatal("settings not preserved")}
 if got,ok:=s.GetUser(user.ID);!ok||got.Key!=user.Key{t.Fatal("settings changed existing credentials")}
}
