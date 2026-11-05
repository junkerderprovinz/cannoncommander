#!/bin/sh
# Starts CannonCommander as PUID:PGID with UMASK, the way Unraid containers
# do it. PUID=0 keeps it running as root.
set -eu

PUID="${PUID:-99}"
PGID="${PGID:-100}"
UMASK="${UMASK:-000}"

case "$PUID$PGID" in
  *[!0-9]*) echo "PUID and PGID must be numbers" >&2; exit 1 ;;
esac
case "$UMASK" in
  [0-7][0-7][0-7] | [0-7][0-7][0-7][0-7]) ;;
  *) echo "UMASK must be three or four octal digits, such as 000 or 022" >&2; exit 1 ;;
esac
umask "$UMASK"

config_dir="$(dirname "$CC_CONFIG")"
mkdir -p "$config_dir/cache"
if [ ! -f "$CC_CONFIG" ]; then
  cp /app/config.default.yaml "$CC_CONFIG"
fi

# Remotes are mounted below /mnt/remotes. A folder that is already there,
# made by Unraid's Unassigned Devices say, keeps its owner.
if [ ! -d /mnt/remotes ]; then
  mkdir -p /mnt/remotes && chown "$PUID:$PGID" /mnt/remotes
fi

as_user=""
if [ "$PUID" != 0 ]; then
  # fusermount3 refuses to mount a remote for a user without a name. Unraid
  # calls 99 nobody, and the line goes first so that name finds 99 rather
  # than Alpine's own nobody.
  if ! cut -d: -f3 /etc/passwd | grep -qx "$PUID"; then
    name=cannoncommander
    [ "$PUID" = 99 ] && name=nobody
    sed -i "1i $name:x:$PUID:$PGID::/config:/sbin/nologin" /etc/passwd
  fi
  if ! cut -d: -f3 /etc/group | grep -qx "$PGID"; then
    echo "cannoncommander:x:$PGID:" >> /etc/group
  fi
  # Only what the server writes itself, never the shares.
  find "$config_dir" \( ! -user "$PUID" -o ! -group "$PGID" \) -exec chown "$PUID:$PGID" {} +
  as_user="su-exec $PUID:$PGID"
fi

# The server runs as a child rather than in place of this script, so the
# banner can be the last block of the startup log, printed once the server
# answers. tini hands a stop to this script and the trap passes it on.
$as_user /app/cannoncommander "$@" &
pid=$!
trap 'kill -TERM "$pid" 2>/dev/null' TERM INT

until curl -fsS -o /dev/null http://localhost:80/health 2>/dev/null; do
  kill -0 "$pid" 2>/dev/null || break
  sleep 1
done

if kill -0 "$pid" 2>/dev/null; then
  version="$(/app/cannoncommander version | sed -n 's/^[[:space:]]*Version[[:space:]]*:[[:space:]]*//p')"
  version="${version:-dev}"
  /app/print-banner.sh "CannonCommander $version" "two-pane file manager for Unraid"
  printf '  \033[0;32m✓ CANNONCOMMANDER %s IS READY\033[0m - Open the WebUI now (port 80)\n\n' "$version"
fi

status=0
wait "$pid" || status=$?
# A signal ends the wait before the server is done, so wait again for its own
# exit status.
while kill -0 "$pid" 2>/dev/null; do
  status=0
  wait "$pid" || status=$?
done
exit "$status"
