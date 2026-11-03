// Round 11: songs and films start playing in quick look, a fine waveform, square play buttons.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(800);
  for (const name of ['album.flac', 'Big.Buck.Bunny.2008.1080p.mkv']) {
    await page.evaluate((n) => { Files.go('/mnt/user/downloads', 0); Files.active = 0; const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === n); CC.render(); $('.panel.active .list').focus(); }, name);
    await page.keyboard.press(' '); await wait(1200);
    console.log(name, 'playing:', await page.locator('.ql .playing').count(), 'time:', await page.locator('.ql .cv-now').first().textContent(), 'bars:', await page.locator('.ql .cv-wave i').count());
    await shot('r11-' + name.split('.')[0]);
    await page.keyboard.press('Escape'); await wait(200);
  }
  console.log('play button radius:', await page.evaluate(() => { CC.go('files'); const b = document.createElement('button'); b.className = 'btn icon cv-playbtn'; document.body.appendChild(b); const r = getComputedStyle(b).borderRadius; b.remove(); return r; }));
};
