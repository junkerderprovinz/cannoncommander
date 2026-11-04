FROM gtstef/ffmpeg:9.0.2-decode AS ffmpeg
FROM golang:alpine AS base
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

FROM node:24-slim AS nbuild
WORKDIR /build/frontend
COPY ./frontend/package.json ./frontend/package-lock.json ./
RUN npm ci
COPY ./frontend/ ./
RUN npm run build:docker

FROM alpine:latest
COPY --from=ffmpeg [ "/ffmpeg", "/ffprobe", "/usr/local/bin/" ]
RUN apk --no-cache add ca-certificates mailcap tzdata curl su-exec 7zip
# Unraid's own defaults: nobody:users, and files anyone on the network may change.
ENV PUID=99 \
    PGID=100 \
    UMASK=000 \
    CC_FFMPEG_PATH="/usr/local/bin/" \
    CC_CONFIG="/config/config.yaml" \
    CC_DATABASE_PATH="/config/cannoncommander.sqlite"
WORKDIR /app
COPY --from=base [ "/app/cannoncommander", "./" ]
COPY [ "./_docker/config.yaml", "./config.default.yaml" ]
COPY [ "./_docker/entrypoint.sh", "./" ]
COPY --from=nbuild [ "/build/frontend/dist/", "./internal/web/dist/" ]
RUN chmod 0755 ./entrypoint.sh && ./cannoncommander version && ffmpeg -version > /dev/null && ffprobe -version > /dev/null && 7zz i > /dev/null
VOLUME /config
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -fsS http://localhost:80/health || exit 1
ENTRYPOINT [ "./entrypoint.sh" ]
