//go:build linux

package panel

import (
	"net"
	"os"
	"testing"
)

// CI invokes this in an isolated network namespace under the exact service
// address-family allowlist. A proc fallback must not conceal broken Netlink
// permissions, since the raw-packet reader needs interface enumeration too.
func TestServiceProfileIPv6Discovery(t *testing.T) {
	expected := os.Getenv("NAMEDICMP_TEST_EXPECT_IPV6")
	if expected == "" {
		t.Skip("requires isolated systemd network fixture")
	}
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) == 0 {
		t.Fatalf("interface enumeration unavailable in service sandbox: %v", err)
	}
	for _, iface := range interfaces {
		if _, err := iface.Addrs(); err != nil {
			t.Fatalf("interface addresses unavailable in service sandbox: %v", err)
		}
	}
	for _, address := range detectedAddresses()["ipv6"] {
		if address == expected {
			return
		}
	}
	t.Fatal("configured fixture IPv6 was not detected inside the service sandbox")
}
