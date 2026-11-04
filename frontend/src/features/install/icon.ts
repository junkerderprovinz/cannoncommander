import { glyph } from '@/core/glyphs';

// The folder glyph in dark ink on the accent colour, drawn on a full square so a round mask can crop it.
export function iconUrl(accent: string) {
  const d = glyph('IconFolderOpen');
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><rect width="512" height="512" rx="112" fill="${accent}"/><svg x="116" y="116" width="280" height="280" viewBox="${d.viewBox}" fill="#161616" color="#161616">${d.svg}</svg></svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}
