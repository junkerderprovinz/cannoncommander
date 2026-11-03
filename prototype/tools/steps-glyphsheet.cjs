// A sheet of the drawn glyphs next to GlimStone's own file glyphs, large and small, for a visual check.
module.exports = async (page, shot) => {
  await page.evaluate(() => {
    const extra = ['IconFileRaw', 'IconFileDesign', 'IconFilePlaylist', 'IconFilePdf', 'IconFileDoc', 'IconFileSheet', 'IconFileSlides', 'IconBook', 'IconDisc', 'IconFileProgram', 'IconFont', 'IconHash', 'IconMagnet', 'IconCube', 'IconCalendar', 'IconHourglass'];
    const own = ['IconFile', 'IconFileImage', 'IconFileVideo', 'IconFileAudio', 'IconArchive', 'IconCode', 'IconFolder', 'IconDatabase'];
    const cell = (n) => `<div style="display:flex;flex-direction:column;align-items:center;gap:6px;width:120px;color:var(--accent-ink)"><span style="width:56px;height:56px;display:block">${g(n)}</span><span style="display:flex;gap:8px"><span style="width:16px;height:16px;display:block">${g(n)}</span><span style="width:20px;height:20px;display:block;color:var(--carbon-text)">${g(n)}</span></span><small style="color:var(--carbon-text-sub);font:12px system-ui">${n.replace('Icon', '')}</small></div>`;
    const d = document.createElement('div');
    d.style.cssText = 'position:fixed;inset:0;z-index:99999;background:var(--carbon-bg);padding:24px;display:flex;flex-direction:column;gap:24px';
    d.innerHTML = `<div style="display:flex;flex-wrap:wrap;gap:16px">${extra.map(cell).join('')}</div><hr style="width:100%;opacity:.2"><div style="display:flex;flex-wrap:wrap;gap:16px">${own.map(cell).join('')}</div>`;
    d.querySelectorAll('svg').forEach(s => { s.style.width = '100%'; s.style.height = '100%'; });
    document.body.appendChild(d);
  });
  await page.waitForTimeout(300);
  await shot('glyphsheet');
};
