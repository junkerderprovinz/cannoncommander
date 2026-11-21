/**
 * Renders the Community Applications icon and the desktop app's icons from
 * logo.svg, the logo alone as every store and taskbar shows it, and the web
 * app's icons from cannoncommander-app.svg, the logo on its white tile:
 *
 *   .github/assets/icon.png                        512, the Community Applications icon
 *   frontend/public/icons/favicon.svg              the tile itself, for the browser tab
 *   frontend/public/icons/app-192.png, app-512.png the installed app (manifest, purpose any)
 *   frontend/public/icons/app-maskable-512.png     the tile inside the safe zone, for round and squircle masks
 *   frontend/public/icons/apple-touch-icon.png     180, on white, as iOS fills transparent corners with black
 *   desktop/build/appicon.png                      1024, the desktop app's window and AppImage
 *   desktop/icons/icon.ico                         the Windows program, installer and shortcuts, 16 to 256
 *   desktop/icons/icons.icns                       the macOS bundle, 16 to 1024
 *   desktop/icons/tray.png                         64, the tray
 *   desktop/icons/icon-16.png ... icon-48.png      the small sizes of the Flatpak's menu entry
 *   desktop/installer/icon.png                     256, the Linux menu entry the installer writes
 *
 * Up to 48 px, and in the tray, the logo goes without its frame: there the frame fills the corners and reads as a tile
 * behind the cannons.
 *
 * Deps (global): @resvg/resvg-js.
 * Run: node .github/assets/gen-icon.mjs
 */
import { copyFileSync, readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const { Resvg } = require(`${execSync("npm root -g").toString().trim()}/@resvg/resvg-js`);
const __dir = dirname(fileURLToPath(import.meta.url));
const icons = join(__dir, "..", "..", "frontend", "public", "icons");
const desktop = join(__dir, "..", "..", "desktop");

const SRC = join(__dir, "cannoncommander-app.svg");
const tile = readFileSync(SRC, "utf8");
const logo = readFileSync(join(__dir, "logo.svg"), "utf8");

// The two halves of the tile, light on the left and grey on the right. The maskable icon continues them to the edge,
// so a mask that cuts into the padding finds the tile's own colours there.
const TILE_HALVES = ["#ffffff", "#e2e2e2"];

const render = (svg, size, background) => new Resvg(svg, { background, fitTo: { mode: "width", value: size } }).render().asPng();

function emit(path, data) {
  writeFileSync(path, data);
  console.log(`wrote ${path}`);
}

// nested places the tile at (x,y,w,h) in an outer drawing and keeps the tile's own viewBox.
function nested(x, y, w, h) {
  const raw = tile.replace(/<\?xml[^>]*\?>\s*/, "");
  const vb = raw.match(/viewBox="([^"]+)"/)[1];
  return raw.replace(/<svg\b[^>]*>/, `<svg x="${x}" y="${y}" width="${w}" height="${h}" viewBox="${vb}" xmlns="http://www.w3.org/2000/svg">`);
}

emit(join(__dir, "icon.png"), render(logo, 512));
copyFileSync(SRC, join(icons, "favicon.svg"));
console.log(`wrote ${join(icons, "favicon.svg")}`);
for (const size of [192, 512]) emit(join(icons, `app-${size}.png`), render(tile, size));
emit(join(icons, "apple-touch-icon.png"), render(tile, 180, "#ffffff"));
emit(join(desktop, "build", "appicon.png"), render(logo, 1024));
emit(join(desktop, "installer", "icon.png"), render(logo, 256));

// A mask keeps at least the circle of 40 % radius, so the whole tile goes into the square that circle holds.
const S = 512, inner = Math.round(S * 0.8 / Math.SQRT2), at = (S - inner) / 2;
const maskable = `<svg xmlns="http://www.w3.org/2000/svg" width="${S}" height="${S}" viewBox="0 0 ${S} ${S}">
  <rect width="${S / 2}" height="${S}" fill="${TILE_HALVES[0]}"/>
  <rect x="${S / 2}" width="${S / 2}" height="${S}" fill="${TILE_HALVES[1]}"/>
  ${nested(at, at, inner, inner)}
</svg>`;
emit(join(icons, "app-maskable-512.png"), render(maskable, S));

// The logo without its frame, the first group of the drawing.
const bare = logo.replace(/(<g id="logo_v3">\s*)<g>[\s\S]*?<\/g>/, "$1");
const SMALL = 48;
const mark = (size) => render(size <= SMALL ? bare : logo, size);

// An .ico of PNG images, which Windows reads at every size.
function ico(sizes) {
  const images = sizes.map((size) => ({ size, png: mark(size) }));
  const head = Buffer.alloc(6 + 16 * images.length);
  head.writeUInt16LE(1, 2);
  head.writeUInt16LE(images.length, 4);
  let at = head.length;
  images.forEach(({ size, png }, i) => {
    const e = 6 + 16 * i;
    head.writeUInt8(size >= 256 ? 0 : size, e);
    head.writeUInt8(size >= 256 ? 0 : size, e + 1);
    head.writeUInt16LE(1, e + 4);
    head.writeUInt16LE(32, e + 6);
    head.writeUInt32LE(png.length, e + 8);
    head.writeUInt32LE(at, e + 12);
    at += png.length;
  });
  return Buffer.concat([head, ...images.map((i) => i.png)]);
}

// An .icns of PNG entries. The retina entries of 16 and 32 points stand at 32 and 64 pixels but show small, so they
// go without the frame too.
function icns(entries) {
  const parts = entries.map(([type, size, small]) => {
    const png = render(small ? bare : logo, size);
    const head = Buffer.alloc(8);
    head.write(type, 0, "ascii");
    head.writeUInt32BE(png.length + 8, 4);
    return Buffer.concat([head, png]);
  });
  const body = Buffer.concat(parts);
  const head = Buffer.alloc(8);
  head.write("icns", 0, "ascii");
  head.writeUInt32BE(body.length + 8, 4);
  return Buffer.concat([head, body]);
}

const out = join(desktop, "icons");
emit(join(out, "icon.ico"), ico([16, 24, 32, 48, 64, 128, 256]));
emit(join(out, "icons.icns"), icns([
  ["icp4", 16, true], ["icp5", 32, true], ["ic11", 32, true], ["ic12", 64, true],
  ["icp6", 64, false], ["ic07", 128, false], ["ic08", 256, false], ["ic13", 256, false],
  ["ic09", 512, false], ["ic14", 512, false], ["ic10", 1024, false],
]));
emit(join(out, "tray.png"), render(bare, 64));
for (const size of [16, 24, 32, 48]) emit(join(out, `icon-${size}.png`), mark(size));
