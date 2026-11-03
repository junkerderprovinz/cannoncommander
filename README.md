<div align="center">

  <h3>CannonCommander</h3>
  A two-pane file manager for Unraid that runs in your browser.

</div>

## Status

CannonCommander is at an early stage. The interface exists as a clickable prototype in [`prototype/`](prototype), with made-up files and no server behind it. The server side starts from [FileBrowser Quantum](https://github.com/gtsteffaniak/filebrowser), whose code and history this repository carries on. There is no release yet, so please don't install it on a server you care about.

## What it is for

Krusader and Total Commander are fast because you work in two panels side by side and press F5 to copy from one to the other. On a home server, getting that usually means running a whole desktop in a container just to move files. CannonCommander brings the same way of working into the browser, and it knows about Unraid:

- Two panels with tabs, an F-key bar, synchronized browsing and a command search on Ctrl+K.
- Folder compare, batch rename with patterns, checksums, duplicate finder and disk usage.
- Seven views, from details with 87 choosable columns to a photo gallery, and quick look on the space bar.
- Copy and move run as background jobs, with a rule for files that already exist.
- Users, groups, access rules, share links and OIDC sign-in, inherited from Quantum and extended.
- Remote storage through rclone.

## Trying the prototype

The prototype is one HTML file built from plain JavaScript, with no framework and no install step.

```sh
cd prototype
node build.mjs
```

Open `prototype/dist/cannoncommander.html` in a browser.

## Comparison

The rows and the values for the web file managers are taken from the [comparison chart in Quantum's README](https://github.com/gtsteffaniak/filebrowser#comparison-chart). The rows after the divider, and the Krusader and Windows Explorer columns (Windows 11 24H2), come from our own look at their documentation.

✅ yes · ❌ no · 🟡 partly · 🧪 designed in the prototype, not yet working · 🚧 planned · ? not verified · n/a does not apply

| | CannonCommander | Quantum | Filebrowser | Filestash | Nextcloud | Google Drive | FileRun | Krusader | Windows Explorer |
|---|---|---|---|---|---|---|---|---|---|
| Filesystem support | 🧪 | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ |
| Linux | 🚧 | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ❌ |
| Windows | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ✅ |
| Mac | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | 🟡 | ❌ |
| Self hostable | 🚧 | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | 🟡 | ❌ |
| Has stable release | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| S3 support | 🧪 | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| WebDAV support | 🧪 | ✅ | ❌ | ✅ | ✅ | ❌ | ✅ | ✅ | 🟡 |
| FTP support | 🧪 | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | ✅ | 🟡 |
| Dedicated docs site | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🟡 |
| Multiple sources at once | 🧪 | ✅ | ❌ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ |
| Docker image size | n/a | 180 MB (with ffmpeg) | 31 MB | 240 MB (main image) | 250 MB | ❌ | > 2 GB | n/a | n/a |
| Min. memory requirements | n/a | 512 MB | 128 MB | 128 MB (main image) | 512 MB | ❌ | 512 MB | n/a | n/a |
| Has standalone binary | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ? | n/a |
| Price | free | free | free | free | free tier | free tier | $99+ | free | included with Windows |
| Rich media preview | 🧪 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🟡 | ✅ |
| Upload files from the web | 🧪 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Advanced search | 🧪 | ✅ | ❌ | ✅ | configurable | ✅ | ✅ | ✅ | ✅ |
| Indexed search | 🧪 | ✅ | ❌ | ✅ | configurable | ✅ | ✅ | 🟡 | ✅ |
| Content-aware search | 🧪 | ❌ | ❌ | ✅ | configurable | ✅ | ✅ | ✅ | ✅ |
| Custom job support | ❌ | 🚧 | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ❌ |
| Multiple users | 🧪 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Single sign-on support | 🧪 | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| LDAP sign-on support | 🚧 | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Long-lived API key support | 🚧 | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| API documentation page | 🚧 | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | 🟡 |
| Mobile app | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Open source | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ |
| Tags support | ❌ | 🚧 | ❌ | ✅ | ✅ | ❌ | ✅ | ? | 🟡 |
| Shareable web links | 🧪 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | 🟡 |
| Event-based notifications | ❌ | 🚧 | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Metrics | ❌ | 🚧 | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| File space quotas | 🚧 | ✅ | ❌ | ✅ | ❌ | ✅ | ✅ | ❌ | 🟡 |
| Text-based files editor | 🧪 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| Office file support | 🧪 | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Office file previews | 🧪 | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ? | 🟡 |
| Themes | 🧪 | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | 🟡 |
| Branding support | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Activity log | 🧪 | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | 🟡 |
| Comments support | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | 🟡 |
| Trash support | 🧪 | 🚧 | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Starred/pinned files | 🧪 | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | 🟡 | ✅ |
| Chromecast support | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Share collections of files | ❌ | 🚧 | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Can archive selected files | 🧪 | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
| Can browse archive files | 🧪 | 🚧 | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ |
| Can convert documents | ❌ | 🚧 | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Can convert videos | ❌ | 🚧 | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Can convert photos | ❌ | 🚧 | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Commander features** | | | | | | | | | |
| Two-pane view | 🧪 | ❌ | ❌ | ? | ❌ | ❌ | ❌ | ✅ | ❌ |
| Tabs | 🧪 | ❌ | ❌ | ? | ❌ | ❌ | ❌ | ✅ | ✅ |
| Function-key bar, keyboard first | 🧪 | 🟡 | ? | 🟡 | ? | 🟡 | ❌ | ✅ | 🟡 |
| Folder compare | 🧪 | ❌ | ❌ | ? | ❌ | ❌ | ❌ | ✅ | ❌ |
| Synchronized browsing | 🧪 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
| Batch rename with patterns | 🧪 | ❌ | ❌ | ? | ? | ❌ | ❌ | ✅ | 🟡 |
| Create and verify checksums | 🧪 | 🟡 | 🟡 | ? | ? | ❌ | ? | ✅ | ❌ |
| Quick look on the space bar | 🧪 | ? | ? | ? | ? | 🟡 | ❌ | 🟡 | 🟡 |
| Choosable list columns | 🧪 | ? | ❌ | ? | ? | ? | ✅ | ✅ | ✅ |
| Thumbnail or gallery view | 🧪 | ✅ | ✅ | ✅ | ✅ | ✅ | ? | ? | ✅ |
| Command search | 🧪 | ? | ❌ | ❌ | ❌ | ❌ | ❌ | ? | ❌ |
| Duplicate finder | 🧪 | ✅ | ❌ | ? | ? | ❌ | ? | ? | ❌ |
| Disk usage view | 🧪 | ✅ | ❌ | ? | 🟡 | 🟡 | ? | ✅ | 🟡 |

## License

CannonCommander is released under the [GNU Affero General Public License v3.0](LICENSE). It builds on FileBrowser Quantum and the original File Browser project, both under the Apache License 2.0; their notices are kept in [NOTICE](NOTICE) and the license text in [LICENSES/Apache-2.0.txt](LICENSES/Apache-2.0.txt). Thank you to Graham Steffaniak and everyone who worked on File Browser before us.
