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

if [ "$PUID" = 0 ]; then
  exec /app/cannoncommander "$@"
fi

# Only what the server writes itself, never the shares.
find "$config_dir" \( ! -user "$PUID" -o ! -group "$PGID" \) -exec chown "$PUID:$PGID" {} +
exec su-exec "$PUID:$PGID" /app/cannoncommander "$@"
