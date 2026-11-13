# Marks on the download buttons and the provider tiles, and where they come from

`<name>.txt` holds the path data of a mark, one path per line, and
`<name>.box.txt` the viewBox it was drawn in. `../gen_download_buttons.py`
reads both and draws every path in the button's one ink.

## Sources and licences

**Font Awesome Free 6.7.2**, from <https://fontawesome.com>: `docker.txt`,
`windows.txt`, `apple.txt` and `linux.txt` from the `brands` set and `zip.txt`,
which is `file-zipper` from the `solid` set. The icons are **CC BY 4.0**, which
asks for attribution and nothing else. Copyright 2024 Fonticons, Inc.

**Simple Icons** (<https://simpleicons.org>): `flatpak.txt`. The path is
**CC0 1.0**, and Flatpak's mark itself is **CC BY 3.0**
(<https://flatpak.org/press>).

**Dashboard Icons** (`homarr-labs/dashboard-icons`,
<https://dashboardicons.com>): `unraid.txt`, its `unraid.svg` with the gradient
left out, since the button draws it in one ink, and `umbrel.txt`. **Apache-2.0**, whose text is
beside this file as `LICENSE-dashboard-icons.txt`. Copyright the Homarr Labs
team and contributors.

`mos.txt` and `zimaos.txt` are the marks of MOS and ZimaOS, the same paths the
other repositories with these buttons draw.

## Provider logos

The tiles of the remotes window carry the provider marks of ArrowLoop, the
sibling app by the same author, whose `scripts/brand-paths/README.md` records
each source. `../brand_marks.mjs` turns ArrowLoop's
`web/src/components/brandGlyphs.tsx` into SVG markup in
`frontend/src/features/config/brandMarks.ts` and copies its
`web/src/brandGlyphs.css`, the colours a mark shifts on one of the two
grounds.

- **Simple Icons** (<https://simpleicons.org>), **CC0 1.0**, for most of them.
- **Dashboard Icons**, Apache-2.0 as above, for OneDrive, Oracle Cloud and
  premiumize.me.
- **The brands' own material**, from their websites and repositories, for the
  marks neither set carries, such as OpenCloud, Drime, FileLu, Shade,
  ImageKit, QingCloud, Sia and 1Fichier. FileLu's and 1Fichier's exist only
  as PNG and were traced. Their use here rests on the trademark position
  below, not on a licence.

Oracle Object Storage wears the Oracle Cloud mark, Yandex Disk Yandex Cloud's,
Huawei Drive Huawei Cloud's and QingStor QingCloud's: each names the right
owner but a broader product than the tile does. A mark that would name another
company's product is left out, and that provider shows a glyph instead.

## Trademarks

Docker, Unraid, MOS, ZimaOS, Umbrel, Windows, Apple, Linux, Flatpak and every
provider mark are trademarks of their owners. Each mark sits on the button for
that platform or the tile that connects to that service, unmodified apart from
its colour, to name it, and nothing here claims endorsement by or affiliation
with any of them.
