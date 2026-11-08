# Contributing to CannonCommander

Thanks for taking a look! This page explains how the repository fits together and how to run it locally.

## What you need

- Go 1.27 (see `backend/go.mod`)
- Node.js 24 with npm (see `frontend/package.json`)
- Git Bash on Windows, so the makefile can run

Some features call outside tools and stay off without them: ffmpeg and ffprobe for media details and video thumbnails, MuPDF for PDF pages, 7-Zip (`7zz`) for 7z archives and rclone for remote storage. The image from `_docker/Dockerfile` ships all of them.

## Layout

- `backend/` is the Go server. `cmd/` starts it, `internal/api` puts the router together, `internal/api/v1` holds what every endpoint shares, and each feature lives in its own package under `internal/` with a `Routes` function (for example `internal/fileops`, `internal/shares`, `internal/accounts`).
- `frontend/` is the Vue 3 interface. It only talks to `/api/v1`. `frontend/src/api/` wraps every call.
- `prototype/` is the clickable HTML prototype the interface is measured against.
- `testing/` builds an Unraid-like fixture tree for tests.

## Running it

```bash
make setup         # dev tools, npm packages and backend/test_config.yaml
make run           # builds the interface, then starts the server with test_config.yaml
```

For work on the interface, start the server and then run `npm run dev` in `frontend/`. Vite sends `/api` on to `http://localhost:8080`, or to `CC_BACKEND` when that is set. Without a server, `npm run dev:fixture` answers `/api/v1` from the prototype's test data.

## Checks

```bash
make test          # go test -race ./... for both builds of the server, and the Vitest suite
make lint          # golangci-lint for both builds, and the type check
```

The server is built twice from the same code. The container leaves out the desktop app's parts, and the desktop app adds them with the `desktop` build tag. A package only the app has puts `//go:build desktop` on its files and keeps a `container.go` beside them, which serves its contract and answers `desktopOnly`. The interface does the same with `desktop.ts` files, which `npm run build:docker` leaves out.

Some Go tests need Linux (inotify, Unraid's files under `/var/local/emhttp`), so CI runs them on Ubuntu. CI also runs them as a normal user, and root passes permission checks that fail there, so `testing/go-test-nonroot.sh` runs `go vet` and `go test -race` as a normal user in a `golang` container, with the argument `desktop` for the app's build; how to start it is at the top of the script. After a change to an API contract, regenerate the OpenAPI document with `go run ./scripts/openapi` in `backend/`.

## Pull requests

Please keep a pull request to one topic and say why the change is needed. Run `make test` and `make lint` before you open it. We look forward to it!
