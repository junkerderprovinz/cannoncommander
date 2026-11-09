# Changelog

Every release is written by hand. The full notes for each release are in
[`.github/release-notes/`](.github/release-notes/) and on the
[releases page](https://github.com/junkerderprovinz/cannoncommander/releases).

## 0.0.0 - 2026-11-05

The first release of CannonCommander: a two-pane file manager for Unraid that runs in the browser, with background jobs, a trash, the tools of Krusader and Total Commander, and an eye on the cache, the array and sleeping disks.

## ✨ Added

- **Two panels with tabs**, a menu bar, a command search on Ctrl+K and the function keys of a commander: F3 views, F4 edits, F5 copies, F6 moves, F7 makes a folder, F8 deletes. The function key bar, browsing both panels in sync, window profiles and your own key bindings are in the View menu and the settings.
- **Copies, moves and deletes run as jobs** in the background, with speed, time left, pause, a speed limit and a rule for files that already exist. A copy can be checked against the original, and jobs carry on after a restart.
- **A trash on every share** and an undo for the last actions. Deleted files wait in the trash for 30 days unless you set another time.
- **Unraid in view.** Files still on the cache carry a badge, a filter shows only those, and the footer says whether a folder lives on the cache or the array. A copy warns before it wakes a sleeping disk or mixes a disk share with a user share, the parity check shows in the sidebar, and "Set permissions the Unraid way" hands files to nobody:users. Admins can gather a folder onto one disk or empty a disk onto the others.
- **Nine views per tab**: details with 89 columns to choose from, compact, list, content, columns, tiles, large tiles, a gallery and a drives view. Quick look opens on the space bar, and Ctrl+Q shows a preview in the other panel.
- **Tools**: search by name, field or content with saved searches, folder compare and sync, file compare, duplicates, disk usage as a tree or a treemap, batch rename with patterns and templates, archives to pack, unpack and test (zip with AES, tar, 7z and rar to read), checksums, split and join, links, and properties with permissions.
- **A viewer and an editor** for pictures, video, audio, PDF, Markdown, text and code, Office files, e-books, 3D models, disk images and archives. OnlyOffice, Euro-Office and Collabora can edit Office files in the browser.
- **Remote storage through rclone**: SFTP, SMB, WebDAV, S3, Google Drive and the rest, connected in the settings and mounted inside the container.
- **Sharing** with links that carry a password, an expiry date and download or upload limits, for a single file or a collection, with a QR code. A WebDAV server serves your files as a drive, with the same users and rules.
- **Users, groups and access rules** that hide a folder or make it read only, two-factor sign-in with recovery codes, passkeys, API tokens that can be read only, single sign-on through OIDC, and an activity log.
- **Colours and tags for files**, frequent folders, shortcuts and pictures for folders, and a link to [ArrowLoop](https://github.com/junkerderprovinz/arrowloop) that turns a pair of folders from the comparison into a recurring sync.
- **A container image** for amd64 and arm64 with ffmpeg, MuPDF, rclone and 7-Zip inside, and an Unraid template. It runs as nobody:users with Unraid's file permissions by default.
- **A desktop app for Windows, macOS and Linux**: the same interface in a window of its own, on your own folders and drives, with nobody to sign in. Typing `cmd`, `powershell`, `pwsh`, `wt` or `terminal` into the path field opens your computer's terminal in that folder. It updates itself once a day, checks every download against the release's checksums and puts the new version in place for the next start.

## 🎨 Design

- **The look is [GlimStone](https://github.com/junkerderprovinz/glimstone) 4.1.1**, shared with BombVault, KnightLoader and ArrowLoop: light and dark, your own accent colour or a rainbow, corner shapes, motion levels and labels as text, symbols or both.
- **Works on the phone** too, with a menu button and a bar at the bottom, and installs as an app.

## 🌐 Translations

- **English and German.**
