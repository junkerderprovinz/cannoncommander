#!/bin/sh
# testing/remote.sh up|down: runs the fixture container on an Unraid test server over SSH.
# CC_TEST_SSH holds the ssh command, for example "ssh -p 22 root@192.168.x.x".
# Everything stays in CC_TEST_DIR on the server, a folder of the test share and never a real one.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
ssh=${CC_TEST_SSH:?set CC_TEST_SSH to the ssh command for the test server}
dir=${CC_TEST_DIR:-/mnt/user/test/cannoncommander}

case "${1:-up}" in
  up)
    $ssh "rm -rf '$dir/src' && mkdir -p '$dir/src'"
    tar -C "$here" -cf - container emhttp fixture generate | $ssh "tar -C '$dir/src' -xf -"
    $ssh "sh '$dir/src/container/run.sh' up '$dir/fixture'"
    ;;
  down)
    $ssh "sh '$dir/src/container/run.sh' down '$dir/fixture' && rm -rf '$dir'"
    ;;
  *) echo "usage: $0 up|down" >&2; exit 2 ;;
esac
