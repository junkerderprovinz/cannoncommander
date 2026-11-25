#!/bin/sh
# Runs the entrypoint the way an unprivileged LXC on Proxmox does: as root in
# a user namespace whose uid 0 is uid 100000 on the host, with the image's
# files shifted by the same amount, as Proxmox shifts the rootfs of such a
# container. Needs a privileged container and starts as its root.
#
# /config belongs to the container, except the paths in HOST_ROOT_OWNS, which
# stay 0:0 with mode 700, like the lost+found mke2fs leaves at the root of a
# Proxmox volume. Afterwards OWNS lines report who owns what, and /config goes
# back to OWNER so the test can read and delete it.
set -u

base=100000

cp /lxc/server-stub.sh /app/cannoncommander
if [ -f /lxc/entrypoint.sh ]; then
  cp /lxc/entrypoint.sh /app/entrypoint.sh
fi
# Docker's own files in /etc are bind mounts and may refuse, which the
# entrypoint does not need.
for d in /etc /app /remotes /usr /bin /sbin /lib /root /tmp /var; do
  chown -R -h "$base:$base" "$d" 2>/dev/null
done
chown -R "$((base + 99)):$((base + 100))" /remotes
chown -R "$base:$base" /config
for p in ${HOST_ROOT_OWNS:-}; do
  mkdir -p "$p"
  chown 0:0 "$p"
  chmod 700 "$p"
done

# A placeholder holds the namespace while its map is written. The entrypoint
# joins it afterwards: a process that runs before the map exists loses its
# capabilities on exec.
unshare -U sleep 600 &
holder=$!
until [ "$(readlink "/proc/$holder/ns/user")" != "$(readlink /proc/self/ns/user)" ]; do sleep 0.1; done
echo "0 $base 65536" > "/proc/$holder/uid_map"
echo "0 $base 65536" > "/proc/$holder/gid_map"
nsenter -U -t "$holder" -S 0 -G 0 /app/entrypoint.sh
rc=$?
kill "$holder"

find /config -maxdepth 1 -exec stat -c 'OWNS %u:%g %n' {} +
chown -R "$OWNER" /config
exit "$rc"
