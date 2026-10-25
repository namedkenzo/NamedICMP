package panel

import("net/http/httptest";"net/http";"path/filepath";"strings";"testing";"time";"encoding/json";"net/netip";"namedicmp/internal/store")
func TestAddressDetectionRejectsPrivateSpecialAndNATRanges(t *testing.T){
 for _,value:=range []string{"127.0.0.1","10.0.0.1","100.64.0.1","192.0.2.1","198.18.0.1","::1","fc00::1","fe80::1","2001:db8::1"}{if publicAddress(netip.MustParseAddr(value)){t.Fatal("not a public detection candidate",value)}}
 for _,value:=range []string{"8.8.8.8","2001:4860:4860::8888"}{if !publicAddress(netip.MustParseAddr(value)){t.Fatal("public address rejected")}}
}
func TestConnectionSettingsRequireAuthCSRFAndPreserveFamily(t *testing.T){
 s,err:=store.Open(filepath.Join(t.TempDir(),"state.json"));if err!=nil{t.Fatal(err)};defer s.Close()
 conf:=store.DefaultConfig();conf.PublicHost="192.0.2.10";p:=New(s,nil,conf)
 request:=func(method,path,body,token string)*httptest.ResponseRecorder{r:=httptest.NewRequest(method,path,strings.NewReader(body));r.Header.Set("Content-Type","application/json");if token!=""{r.AddCookie(&http.Cookie{Name:"namedicmp_session",Value:"session"});r.Header.Set("X-CSRF-Token",token)};w:=httptest.NewRecorder();p.Handler().ServeHTTP(w,r);return w}
 if w:=request("GET","/api/settings/connection","","");w.Code!=401{t.Fatal("settings exposed without login")}
 p.sessions["session"]=Session{CSRF:"csrf",Expires:time.Now().Add(time.Hour)}
 if w:=request("PUT","/api/settings/connection",`{"family":"ipv6","host":"2001:db8::10","reader_mode":"raw_ip"}`,"wrong");w.Code!=403{t.Fatal("CSRF not enforced")}
 for _,host:=range []string{"2001:db8::10","vpn.example.com",""}{
  body:=`{"family":"ipv6","host":"`+host+`","reader_mode":"raw_packet"}`
  w:=request("PUT","/api/settings/connection",body,"csrf");if w.Code!=200{t.Fatal(w.Body.String())}
  var result map[string]any;if json.Unmarshal(w.Body.Bytes(),&result)!=nil||result["family"]!="ipv6"{t.Fatal("family lost")}
 }
 w:=request("PUT","/api/settings/connection",`{"family":"ipv6","host":"192.0.2.5","reader_mode":"raw_ip"}`,"csrf");if w.Code!=400{t.Fatal("wrong-family address accepted")}
}
