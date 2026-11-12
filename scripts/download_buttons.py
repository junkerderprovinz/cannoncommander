"""The download buttons the README shows, read by gen_download_buttons.py.

Each entry names a button the generator knows and where it leads. The rows,
their order, the colours and the words are the generator's, the same in every
repository.
"""

REPO = "cannoncommander"
RELEASE = "https://github.com/junkerderprovinz/cannoncommander/releases/latest/download/"

BUTTONS = {
    # The desktop app comes with every release from the first one on, under
    # these names without the version (.github/workflows/release.yml).
    "windows": RELEASE + "cannoncommander-windows-amd64-installer.exe",
    "windows-arm": RELEASE + "cannoncommander-windows-arm64-installer.exe",
    "macos": RELEASE + "cannoncommander-macos-universal.zip",
    "linux": RELEASE + "cannoncommander-linux-amd64-installer",
    "linux-arm": RELEASE + "cannoncommander-linux-arm64-installer",
    "flatpak": RELEASE + "cannoncommander-linux-amd64.flatpak",
    "flatpak-arm": RELEASE + "cannoncommander-linux-arm64.flatpak",
    # Not in Community Applications yet, so the button is drawn without a link
    # and says "coming soon". The listing's address goes here once it exists.
    "unraid": None,
    "compose": "https://raw.githubusercontent.com/junkerderprovinz/cannoncommander/main/compose.yaml",
    # A release's "Source code (zip)" is the whole repository at that tag, and
    # GitHub gives the newest one no fixed address, so this leads to the release
    # that lists it.
    "source": "https://github.com/junkerderprovinz/cannoncommander/releases/latest",
}
