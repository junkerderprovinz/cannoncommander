# CannonCommander: a two-pane file manager for Unraid in the browser
#
# GitHub:  https://github.com/junkerderprovinz/cannoncommander
# Image:   ghcr.io/junkerderprovinz/cannoncommander
# License: AGPL-3.0-only
#
# One Go server with the Vue interface embedded, plus the tools it calls for
# media details, PDF pages, 7z archives and remote storage.

ARG BUILDPLATFORM

FROM gtstef/ffmpeg:9.0.2-decode AS ffmpeg
FROM golang:1.27-alpine AS base
# CI stamps the git describe and the commit. Without them the binary reads
# both from backend/internal/version/archive.txt, which git archive fills in,
# and a plain checkout calls itself dev.
ARG VERSION
ARG REVISION
WORKDIR /app
COPY ./backend ./
RUN apk update && apk add --no-cache gcc musl-dev
RUN go mod download && go run ./scripts/setup-gofitz-cgo
ENV CGO_ENABLED=1
RUN go build -tags mupdf,musl -ldflags="-w -s \
  -X 'github.com/junkerderprovinz/cannoncommander/backend/internal/version.Version=${VERSION}' \
  -X 'github.com/junkerderprovinz/cannoncommander/backend/internal/version.CommitSHA=${REVISION}'" \
  -o cannoncommander .

# The interface is the same for every platform, so it builds once on the
# runner's own one instead of under emulation.
FROM --platform=$BUILDPLATFORM node:24-slim AS nbuild
WORKDIR /build/frontend
COPY ./frontend/package.json ./frontend/package-lock.json ./
RUN npm ci
COPY ./frontend/ ./
RUN npm run build:docker

FROM alpine:3.24
LABEL org.opencontainers.image.title="cannoncommander" \
      org.opencontainers.image.description="A two-pane file manager for Unraid in the browser." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/cannoncommander" \
      org.opencontainers.image.licenses="AGPL-3.0-only"
COPY --from=ffmpeg [ "/ffmpeg", "/ffprobe", "/usr/local/bin/" ]
# tini reaps the rclone processes the server leaves behind if it dies.
RUN apk --no-cache add ca-certificates mailcap tzdata curl setpriv 7zip tini
# rclone mounts the remotes below /remotes, outside the /mnt the container
# shares with the host, so they stay inside the container.
RUN apk --no-cache add rclone fuse3 && mkdir /remotes && chown 99:100 /remotes
# The terminal in the browser gets bash, and the terminfo of the xterm it
# announces itself as.
RUN apk --no-cache add bash ncurses-terminfo-base
# RAR packs .rar archives. RARLAB builds it for x86-64 Linux alone, against
# glibc, and asks that the package travel unchanged, licence included.
ARG TARGETARCH
ARG RAR_VERSION=723
ARG RAR_SHA256=759b4b6aa0d9f77131882162951193f3a0e54bf60e1d8dc4255aa308accab588
RUN if [ "$TARGETARCH" = "amd64" ]; then \
      apk --no-cache add gcompat libstdc++ \
      && curl -fsSLo /tmp/rar.tar.gz "https://www.rarlab.com/rar/rarlinux-x64-${RAR_VERSION}.tar.gz" \
      && echo "${RAR_SHA256}  /tmp/rar.tar.gz" | sha256sum -c - \
      && tar xzf /tmp/rar.tar.gz -C /opt && rm /tmp/rar.tar.gz \
      && ln -s /opt/rar/rar /usr/local/bin/rar && rar -iver > /dev/null; \
    fi
# Unraid's own defaults: nobody:users, and files anyone on the network may change.
ENV PUID=99 \
    PGID=100 \
    UMASK=000 \
    LANG=C.UTF-8 \
    CC_FFMPEG_PATH="/usr/local/bin/" \
    CC_CONFIG="/config/config.yaml" \
    CC_DATABASE_PATH="/config/cannoncommander.sqlite"
WORKDIR /app
COPY --from=base [ "/app/cannoncommander", "./" ]
COPY [ "./_docker/config.yaml", "./config.default.yaml" ]
COPY [ "./_docker/entrypoint.sh", "./_docker/print-banner.sh", "./" ]
COPY [ "./.github/assets/banner-raw.txt", "/usr/local/share/banner-raw.txt" ]
COPY --from=nbuild [ "/build/frontend/dist/", "./internal/web/dist/" ]
# The art is checked out with whatever line endings the build machine uses.
RUN tr -d '\r' < /usr/local/share/banner-raw.txt > /usr/local/share/banner.txt \
  && chmod 0755 ./entrypoint.sh ./print-banner.sh \
  && ./cannoncommander version && ffmpeg -version > /dev/null && ffprobe -version > /dev/null && rclone version > /dev/null && 7zz i > /dev/null
VOLUME /config
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -fsS http://localhost:80/health || exit 1
ENTRYPOINT [ "/sbin/tini", "--", "./entrypoint.sh" ]
