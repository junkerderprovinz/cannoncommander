#!/bin/sh
# testing/container/run.sh up [data dir] | down [data dir]
# Starts the fixture container with its own limits, or removes it, its image and its data again.
set -eu
here=$(cd "$(dirname "$0")/.." && pwd)
name=cc-test-fixture
data=${2:-$here/out/fixture}

case "${1:-up}" in
  up)
    docker build -t "$name" -f "$here/container/Dockerfile" "$here"
    docker rm -f "$name" >/dev/null 2>&1 || true
    mkdir -p "$data"
    # Docker on a host without AppArmor refuses the option, Ubuntu needs it for a FUSE mount inside.
    extra=
    if [ -d /sys/kernel/security/apparmor ]; then extra="--security-opt apparmor=unconfined"; fi
    docker run -d --name "$name" --cpus 2 --memory 2g --memory-swap 2g --pids-limit 512 \
      --device /dev/fuse --cap-add SYS_ADMIN $extra -v "$data:/fixture" "$name"
    # The container is ready once the tree has passed its check.
    i=0
    until docker exec "$name" test -f /fixture/ready 2>/dev/null; do
      if [ "$(docker inspect -f '{{.State.Running}}' "$name")" != true ] || [ $i -ge 600 ]; then docker logs "$name"; exit 1; fi
      i=$((i + 1)); sleep 1
    done
    docker logs "$name"
    ;;
  down)
    docker rm -f "$name" >/dev/null 2>&1 || true
    docker rmi "$name" >/dev/null 2>&1 || true
    rm -rf "$data"
    ;;
  *) echo "usage: $0 up|down [data dir]" >&2; exit 2 ;;
esac
