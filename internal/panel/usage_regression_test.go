package panel

import (
 "testing"
 "namedicmp/internal/store"
)
func TestProfileIncludesTotalUsage(t *testing.T){
 u:=store.User{Upload:100,Download:200,Recovery:50,Encryption:"none"}
 p:=userProfile(u);if p["used"]!=int64(350){t.Fatalf("panel loses usage: %#v",p)}
}
