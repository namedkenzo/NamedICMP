#!/usr/bin/env bash
# Exercise a real systemd stop and port release without touching an installation.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
fixture_dir=$(mktemp -d)
cleanup(){ sudo systemctl stop namedicmp-uninstall-test.service || true; rm -rf -- "$fixture_dir"; }
trap cleanup EXIT
sudo systemd-run --quiet --unit=namedicmp-uninstall-test --service-type=exec \
  --property=Restart=on-failure \
  python3 -c 'import socket,sys,time; s=socket.socket(); s.bind(("127.0.0.1",0)); s.listen(); open(sys.argv[1],"w").write(str(s.getsockname()[1])); time.sleep(60)' "$fixture_dir/port"
for attempt in {1..50};do [[ ! -s $fixture_dir/port ]] || break;sleep 0.1;done
[[ -s $fixture_dir/port ]]
port=$(<"$fixture_dir/port")
python3 - "$port" <<'PY'
import socket,sys
with socket.create_connection(('127.0.0.1', int(sys.argv[1])), timeout=2):
    pass
PY
sudo bash -s -- "$PWD/install.sh" <<'TEST'
set -euo pipefail
export NAMEDICMP_INSTALLER_TEST=1
source "$1"
# The optional timer is intentionally absent, like ordinary HTTP installs.
uninstall_stop_unit namedicmp-uninstall-absent.timer
uninstall_stop_unit namedicmp-uninstall-test.service
TEST
python3 - "$port" <<'PY'
import socket,sys
# A listener which survived stop would prevent rebinding this exact port.
with socket.socket() as listener:
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind(('127.0.0.1', int(sys.argv[1])))
    listener.listen()
print('Real systemd stop with absent optional timer released the listening port.')
PY
