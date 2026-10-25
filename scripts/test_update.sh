#!/usr/bin/env bash
# Test deployment state transitions with an isolated mock service, never systemd.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
export NAMEDICMP_INSTALLER_TEST=1
source ./install.sh
fixture=$(mktemp -d /tmp/namedicmp-update-test.XXXXXXXX)
trap 'rm -rf -- "$fixture"' EXIT
PREFIX="$fixture/opt";STATE="$fixture/state";CONFIG="$fixture/config.json";workdir="$fixture/work"
mkdir -p "$PREFIX" "$STATE" "$workdir" "$fixture/source"
cp install.sh "$fixture/source/install.sh"
mkdir -p "$fixture/source/scripts"
cp scripts/network_tuning.py "$fixture/source/scripts/"
network_tuning(){ :; }
printf '{"users":{"test":{"key":123456,"upload":99}}}\n' >"$STATE/state.json"
python3 - "$CONFIG" "$STATE/state.json" <<'PY'
import json,sys
json.dump({'state_path':sys.argv[2],'panel_listen':'127.0.0.1:8443','tls_cert':'certificate.pem'},open(sys.argv[1],'w'))
PY
cp "$STATE/state.json" "$fixture/state.expected";cp "$CONFIG" "$fixture/config.expected"
cat >"$PREFIX/namedicmp" <<'BIN'
#!/usr/bin/env bash
[[ ${1:-} == version ]] && { echo 0.1.0-alpha;exit 0; }
exit 0
BIN
chmod +x "$PREFIX/namedicmp"
source_tree(){ printf -v "$1" '%s' "$fixture/source"; }
build_candidate(){ cat >"$workdir/namedicmp" <<'BIN'
#!/usr/bin/env bash
case ${1:-} in version)echo 0.2.1-alpha;;diagnostics)[[ ${FAIL_HEALTH:-0} != 1 ]];;*)exit 0;;esac
BIN
chmod +x "$workdir/namedicmp"; }
remember_source(){ :; }
sleep(){ :; }
service_running=y
systemctl(){ case $1 in is-active)[[ $service_running == y ]];;stop)service_running=n;;start)service_running=y;;*)return 0;;esac; }
update_namedicmp
[[ $("$PREFIX/namedicmp" version) == 0.2.1-alpha && $service_running == y ]]
cmp "$CONFIG" "$fixture/config.expected";cmp "$STATE/state.json" "$fixture/state.expected"
service_running=n;update_namedicmp
[[ $service_running == n ]]
# A failed health check must restore the actual previous executable.
printf '#!/usr/bin/env bash\necho previous-stable\n' >"$PREFIX/namedicmp";chmod +x "$PREFIX/namedicmp"
service_running=y;export FAIL_HEALTH=1
if update_namedicmp;then say 'Unhealthy update accepted';exit 1;fi
[[ $("$PREFIX/namedicmp" version) == previous-stable && $service_running == y ]]
cmp "$CONFIG" "$fixture/config.expected";cmp "$STATE/state.json" "$fixture/state.expected"
count=$(find "$STATE/backups" -name state.json | wc -l)
[[ $count -eq 3 ]]
say 'Update preservation, stopped-service handling, unique backups and failed-health rollback passed.'

