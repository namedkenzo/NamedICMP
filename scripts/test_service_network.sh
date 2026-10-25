#!/usr/bin/env bash
# GitHub's Linux runner only: no host interface, route or firewall is changed.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
fixture_dir=$(mktemp -d)
cleanup(){ sudo ip netns del namedicmp-discovery-test || true; rm -rf -- "$fixture_dir"; }
trap cleanup EXIT
chmod 0755 "$fixture_dir"
go test -c -o "$fixture_dir/panel.test" ./internal/panel
sudo ip netns add namedicmp-discovery-test
sudo ip -n namedicmp-discovery-test link set lo up
sudo ip -n namedicmp-discovery-test link add fixture0 type dummy
sudo ip -n namedicmp-discovery-test link set fixture0 up
# No external link exists in this namespace: this address cannot escape it.
fixture_ipv6=2001:4860:6e61:6d65::1
sudo ip -n namedicmp-discovery-test -6 address add "$fixture_ipv6/64" dev fixture0 nodad
families=$(sed -n 's/^RestrictAddressFamilies=//p' install.sh)
[[ " $families " == *' AF_NETLINK '* ]]
sudo systemd-run --quiet --wait --pipe --collect \
  --unit=namedicmp-discovery-test \
  --property=NetworkNamespacePath=/run/netns/namedicmp-discovery-test \
  --property="RestrictAddressFamilies=$families" \
  --property=User=nobody --property=NoNewPrivileges=yes \
  --setenv="NAMEDICMP_TEST_EXPECT_IPV6=$fixture_ipv6" \
  "$fixture_dir/panel.test" -test.run '^TestServiceProfileIPv6Discovery$' -test.v
