package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

func diagnostics(args []string) error {
	f := flag.NewFlagSet("diagnostics", flag.ContinueOnError)
	sock := f.String("socket", "/run/namedicmp/admin.sock", "management socket")
	if err := f.Parse(args); err != nil {
		return err
	}
	client := http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", *sock)
	}}}
	res, err := client.Get("http://unix/diagnostics")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8192))
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("service health status %d", res.StatusCode)
	}
	var pretty bytes.Buffer
	if err = json.Indent(&pretty, body, "", "  "); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, pretty.String())
	return err
}
