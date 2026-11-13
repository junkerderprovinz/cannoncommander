# Building from source

You need Go (the version is in `backend/go.mod`), Node.js 24 with npm, and Docker for the image.

## The container image {#image}

```sh
git clone https://github.com/junkerderprovinz/cannoncommander.git
cd cannoncommander
docker build -t cannoncommander -f _docker/Dockerfile .
```

That builds the same image the releases use, with ffmpeg, MuPDF, rclone and 7-Zip inside.

## Working on the code {#dev}

| Command | Does |
|---|---|
| `make setup` | Installs the tools |
| `make run` | Starts the server with a test configuration |
| `npm run dev:fixture` in `frontend/` | Runs the interface against made-up files, without a server |

[CONTRIBUTING.md](https://github.com/junkerderprovinz/cannoncommander/blob/main/CONTRIBUTING.md) explains the layout and the checks.

The interface against the made-up files is what every picture on this site shows. `node scripts/docs_screenshots.mjs` takes them again, after `npm ci` in `frontend/` and `prototype/`.

## The desktop app {#desktop}

The desktop app builds on the system it is for:

```sh
node scripts/desktop.mjs
```

- `--installer` adds the installer on Windows and Linux.
- `--arch arm64` builds for ARM on Windows.

It needs the Wails CLI in the version `desktop/go.mod` names. Windows needs nothing else. A Mac needs the Xcode tools, and Linux gcc with the GTK 3 and WebKitGTK 4.1 headers.

## This site {#docs}

```sh
pip install -r docs-requirements.txt
mkdocs serve
```

Then open `http://127.0.0.1:8000`. The pages are in `docs/`.
