#!/bin/sh
# Starts the entrypoint of an image the way an unprivileged LXC on Proxmox
# does, with /config on a fresh volume, and the server where ports below
# 1024 need a capability, as in every LXC.
#
#   testing/lxc/test.sh cannoncommander:smoke [entrypoint.sh to try instead]
set -eu

img="${1:?usage: $0 <image> [entrypoint]}"
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mounts="-v $here/lxc-run.sh:/lxc/lxc-run.sh:ro -v $here/server-stub.sh:/lxc/server-stub.sh:ro"
entry=""
if [ -n "${2:-}" ]; then
  entry="$(cd "$(dirname "$2")" && pwd)/$(basename "$2")"
  mounts="$mounts -v $entry:/lxc/entrypoint.sh:ro"
  entry="-v $entry:/app/entrypoint.sh:ro"
fi
failed=0

# boot <case> <paths the host's root owns>: runs the entrypoint once on an
# empty /config and keeps its output in the case's log.
boot() {
  dir="$work/$1"
  shift
  mkdir -p "$dir/config"
  # shellcheck disable=SC2086
  docker run --rm --privileged --entrypoint /bin/sh \
    -e PUID=99 -e PGID=100 -e OWNER="$(id -u):$(id -g)" -e HOST_ROOT_OWNS="$*" \
    $mounts -v "$dir/config:/config" \
    "$img" /lxc/lxc-run.sh > "$dir/log" 2>&1
}

fail() {
  echo "FAIL $1: $2"
  sed 's/^/    | /' "$work/$1/log"
  failed=1
}

owns() {
  grep -qxF "OWNS $2 $3" "$work/$1/log" || fail "$1" "expected $3 owned by $2"
}

if boot lost-found /config/lost+found; then
  grep -qxF "SERVER 99:100" "$work/lost-found/log" || fail lost-found "the server did not start as 99:100"
  owns lost-found 100099:100100 /config
  owns lost-found 100099:100100 /config/config.yaml
  owns lost-found 0:0 /config/lost+found
else
  fail lost-found "the container stopped with the host's lost+found in /config"
fi

if boot foreign /config/lost+found /config/foreign; then
  fail foreign "the container started with a folder it cannot take over"
elif ! grep -q "/config/foreign" "$work/foreign/log"; then
  fail foreign "the log does not name the folder"
elif grep -q "^SERVER " "$work/foreign/log"; then
  fail foreign "the server started"
fi

# Docker lets anyone bind ports below 1024 in a container, an LXC keeps the
# kernel's 1024, and Proxmox does not let a container change that.
port=cc-lxc-port80
dir="$work/port80"
mkdir -p "$dir/config" "$dir/share"
# shellcheck disable=SC2086
docker run -d --name "$port" --sysctl net.ipv4.ip_unprivileged_port_start=1024 \
  -e PUID=1000 -e PGID=1000 $entry -v "$dir/config:/config" -v "$dir/share:/mnt/user" \
  "$img" > /dev/null
up=0
for _ in $(seq 1 60); do
  if docker exec "$port" curl -fsS -o /dev/null http://localhost:80/health 2> /dev/null; then
    up=1
    break
  fi
  [ -n "$(docker ps -q --filter "name=^$port\$")" ] || break
  sleep 1
done
caps=""
if [ "$up" = 1 ]; then
  pid="$(docker exec "$port" pgrep -f '^/app/cannoncommander' | head -n 1)"
  caps="$(docker exec "$port" sh -c "stat -c %u:%g /proc/$pid; grep -E '^Cap(Inh|Amb)' /proc/$pid/status")"
fi
docker logs "$port" > "$work/port80.log" 2>&1
docker rm -f "$port" > /dev/null
# The server wrote /config as 1000, which the runner may not delete.
docker run --rm -v "$work:/work" --entrypoint /bin/sh "$img" -c 'rm -rf /work/port80'
mkdir -p "$dir"
mv "$work/port80.log" "$dir/log"
if [ "$up" = 1 ]; then
  echo "$caps" | grep -qx "1000:1000" || fail port80 "the server does not run as 1000:1000"
  for set in CapInh CapAmb; do
    mask="$(echo "$caps" | sed -n "s/^$set:[[:space:]]*//p")"
    [ $((0x$mask & 0x400)) != 0 ] || fail port80 "net_bind_service is missing from $set ($mask)"
  done
else
  fail port80 "the server did not answer on port 80 as PUID 1000"
fi

[ "$failed" = 0 ] && echo "ok: the entrypoint starts like an unprivileged LXC"
exit "$failed"
