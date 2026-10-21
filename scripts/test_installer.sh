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
[[ $REPOSITORY == namedkenzo/NamedICMP ]]
validate_source
REPOSITORY='invalid/repo/extra'
if validate_source;then say 'Invalid repository accepted';exit 1;fi
REPOSITORY=namedkenzo/NamedICMP
# Release deployment tests use local fixtures and never install on the host.
temporary
fixture="$workdir/release-fixture";mkdir -p "$fixture"
printf '#!/usr/bin/env bash\necho %s\n' "$MANAGER_VERSION" >"$fixture/namedicmp"
chmod +x "$fixture/namedicmp"
asset="namedicmp-linux-$(uname -m).tar.gz"
case $(uname -m) in x86_64)asset=namedicmp-linux-amd64.tar.gz;;aarch64|arm64)asset=namedicmp-linux-arm64.tar.gz;;esac
tar -czf "$fixture/$asset" -C "$fixture" namedicmp
(cd "$fixture";sha256sum "$asset" >SHA256SUMS)
download(){ cp "$fixture/${1##*/}" "$2"; }
release_candidate
[[ $("$workdir/namedicmp" version) == "$MANAGER_VERSION" ]]
# A different executable version must fail even with a valid checksum.
printf '#!/usr/bin/env bash\necho wrong-version\n' >"$fixture/namedicmp"
tar -czf "$fixture/$asset" -C "$fixture" namedicmp
(cd "$fixture";sha256sum "$asset" >SHA256SUMS)
if release_candidate;then say 'Wrong release version accepted';exit 1;fi
printf 'bad checksum\n' >"$fixture/SHA256SUMS"
if release_candidate;then say 'Invalid manifest accepted';exit 1;fi
(cd "$fixture";sha256sum "$asset" >SHA256SUMS)
printf 'corrupted' >>"$fixture/$asset"
if release_candidate;then say 'Corrupted release accepted';exit 1;fi
# Unavailable assets can fall back in auto mode; corrupt assets cannot.
download(){ return 1; }
result=0
release_candidate || result=$?
[[ $result == 2 ]]
INSTALL_MODE=binary
if build_candidate "$resolved_source";then say 'Binary-only install silently fell back';exit 1;fi
# A provisioned machine must not refresh all APT indexes again.
dpkg-query(){ printf 'install ok installed\n'; }
apt-get(){ say 'Unexpected APT call';return 1; }
packages
say 'Default repository, release integrity, binary-only failure and dependency reuse passed.'
# An installed older manager must select the version of the downloaded source.
mkdir -p "$fixture/new-source/cmd/namedicmp"
printf 'const version = "9.8.7-alpha"\n' >"$fixture/new-source/cmd/namedicmp/main.go"
(
 INSTALL_MODE=binary
 release_candidate(){ [[ $MANAGER_VERSION == 9.8.7-alpha ]]; }
 build_candidate "$fixture/new-source"
)
# Auto mode must never compile around an integrity failure; unavailable assets
# may fall back. Load a standalone manager to exercise the remote-install path.
cp install.sh "$fixture/manager.sh"
bash -s -- "$fixture/manager.sh" "$resolved_source" "$fixture" <<'CHILD'
set -euo pipefail
export NAMEDICMP_INSTALLER_TEST=1
source "$1"
INSTALL_MODE=auto
source_dir=$2;fixture_dir=$3
release_candidate(){ return 1; }
go_binary(){ touch "$fixture_dir/fallback-used";return 1; }
if build_candidate "$source_dir";then exit 1;fi
[[ ! -e $fixture_dir/fallback-used ]]
release_candidate(){ return 2; }
# A failed toolchain setup must stop fallback before any build is attempted.
if build_candidate "$source_dir";then exit 1;fi
[[ -e $fixture_dir/fallback-used ]]
CHILD
say 'New-source version selection and safe automatic fallback passed.'
