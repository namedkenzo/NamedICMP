#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
bash -n install.sh
export NAMEDICMP_INSTALLER_TEST=1
source ./install.sh
for port in 1 80 443 8443 65535 08443;do valid_port "$port" || { say "Rejected valid port $port";exit 1; };done
for port in 0 65536 -1 12abc '' 99999999999;do if valid_port "$port";then say "Accepted invalid port $port";exit 1;fi;done
source_tree resolved_source
[[ -f $resolved_source/go.mod && -f $resolved_source/internal/core/core.go ]]
admin_username='test"name';admin_password='test-password-1234'
credentials_json | python3 -c 'import sys,json;d=json.load(sys.stdin);assert d["username"]=="test\"name";assert len(d["password"])>=12'
say 'Installer syntax, port validation, local-source discovery and credential encoding passed.'
PREFIX=/tmp/namedicmp-nonexistent-test-prefix;CONFIG=/tmp/namedicmp-nonexistent-test-config
menu_render | python3 -c 'import sys;s=sys.stdin.read();assert "Independent TCP + UDP" in s;assert "Update" in s and "SSL certificate" in s and "Uninstall" in s;assert "\x1b" not in s'
temporary;marker="$workdir/action-continued"
bad_action(){ false;touch "$marker"; }
run_menu_action bad_action
[[ ! -e $marker ]]
say 'Menu grouping, non-TTY output and strict action-failure handling passed.'
