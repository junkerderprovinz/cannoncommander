#!/bin/sh
# Builds the fixture, mounts it where Unraid keeps these paths, checks it and then runs the command.
set -eu
root=/fixture
rm -f "$root/ready"

if mountpoint -q "$root/mnt/user"; then fusermount3 -u "$root/mnt/user"; fi
node /testing/generate/generate.mjs --root "$root" --clean

# Cache first, then the disks, the order Unraid's shfs reads them in; new files go where most space is left.
branches=$(for d in "$root"/mnt/cache "$root"/mnt/disk*; do printf '%s:' "$d"; done)
mergerfs -o allow_other,cache.files=off,category.create=mfs,fsname=shfs "${branches%:}" "$root/mnt/user"

mount --rbind "$root/mnt" /mnt
mkdir -p /boot /remotes /var/local/emhttp
mount --bind "$root/boot" /boot
mount --bind "$root/remotes" /remotes
mount --bind "$root/var/local/emhttp" /var/local/emhttp
mount --bind "$root/proc/mdstat" /proc/mdstat

node /testing/generate/verify.mjs --root "$root" --list "$root/listing.txt"
touch "$root/ready"
exec "$@"
