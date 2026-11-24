#!/bin/sh
# Starts the entrypoint of an image the way an unprivileged LXC on Proxmox
# does, with /config on a fresh volume.
#
#   testing/lxc/test.sh cannoncommander:smoke [entrypoint.sh to try instead]
set -eu

img="${1:?usage: $0 <image> [entrypoint]}"
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mounts="-v $here/lxc-run.sh:/lxc/lxc-run.sh:ro -v $here/server-stub.sh:/lxc/server-stub.sh:ro"
if [ -n "${2:-}" ]; then
  mounts="$mounts -v $(cd "$(dirname "$2")" && pwd)/$(basename "$2"):/lxc/entrypoint.sh:ro"
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

[ "$failed" = 0 ] && echo "ok: the entrypoint starts like an unprivileged LXC"
exit "$failed"
