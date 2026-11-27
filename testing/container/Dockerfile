# The fixture server: the prototype's disks as a real tree, /mnt/user put together by mergerfs and Unraid's
# emhttp files in their usual places. Build context is testing/.
FROM node:24-trixie-slim@sha256:4d00acb4bcc42c454c4eb801526f7ad7f772ea3bd4fd168c5cc2e384abcbb9d4

RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg mergerfs fuse3 \
 && rm -rf /var/lib/apt/lists/*

COPY generate /testing/generate
COPY emhttp /testing/emhttp
COPY fixture /testing/fixture
COPY container/entrypoint.sh /testing/entrypoint.sh

VOLUME /fixture
ENTRYPOINT ["/bin/sh", "/testing/entrypoint.sh"]
CMD ["sleep", "infinity"]
