package core

import("testing";"time")

func TestEOFNeverDiscardsUnacknowledgedDownload(t *testing.T){
	for _,remote:=range []bool{false,true}{if tcpDrainComplete(true,remote,false,time.Hour){t.Fatal("EOF discarded outstanding download")}}
	if !tcpDrainComplete(true,true,true,time.Second)||!tcpDrainComplete(true,false,true,31*time.Second){t.Fatal("completed stream did not close")}
	if tcpDrainComplete(false,true,true,time.Hour){t.Fatal("stream closed before EOF")}
}
