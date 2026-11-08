/**
 * Renders every icon from cannoncommander-app.svg, the logo on its white tile:
 *
 *   .github/assets/icon.png                        512, the Community Applications icon
 *   frontend/public/icons/favicon.svg              the tile itself, for the browser tab
 *   frontend/public/icons/app-192.png, app-512.png the installed app (manifest, purpose any)
 *   frontend/public/icons/app-maskable-512.png     the tile inside the safe zone, for round and squircle masks
 *   frontend/public/icons/apple-touch-icon.png     180, on white, as iOS fills transparent corners with black
 *   desktop/build/appicon.png                      1024, the desktop app, from which Wails makes the .ico and .icns
 *   desktop/installer/icon.png                     256, the Linux menu entry the installer writes
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

emit(join(__dir, "icon.png"), render(tile, 512));
copyFileSync(SRC, join(icons, "favicon.svg"));
console.log(`wrote ${join(icons, "favicon.svg")}`);
for (const size of [192, 512]) emit(join(icons, `app-${size}.png`), render(tile, size));
emit(join(icons, "apple-touch-icon.png"), render(tile, 180, "#ffffff"));
emit(join(desktop, "build", "appicon.png"), render(tile, 1024));
emit(join(desktop, "installer", "icon.png"), render(tile, 256));

// A mask keeps at least the circle of 40 % radius, so the whole tile goes into the square that circle holds.
const S = 512, inner = Math.round(S * 0.8 / Math.SQRT2), at = (S - inner) / 2;
const maskable = `<svg xmlns="http://www.w3.org/2000/svg" width="${S}" height="${S}" viewBox="0 0 ${S} ${S}">
  <rect width="${S / 2}" height="${S}" fill="${TILE_HALVES[0]}"/>
  <rect x="${S / 2}" width="${S / 2}" height="${S}" fill="${TILE_HALVES[1]}"/>
  ${nested(at, at, inner, inner)}
</svg>`;
emit(join(icons, "app-maskable-512.png"), render(maskable, S));
