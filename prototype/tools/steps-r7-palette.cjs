// Round 7: the command search on Ctrl+K and the settings pages in two columns on a wide screen.
module.exports = async (page, shot) => {
  const wait = (ms = 300) => page.waitForTimeout(ms);
  const rows = () => page.$$eval('.pal-row', rs => rs.map(r => `${r.classList.contains('off') ? '(aus) ' : ''}${r.querySelector('.t').textContent}${r.querySelector('.kbd') ? ' ' + r.querySelector('.kbd').textContent : ''}`));
  await wait(500);

  await page.keyboard.press('Control+k'); await wait();
  console.log('palette open:', await page.locator('.pal').count(), 'focus in search:', await page.evaluate(() => document.activeElement?.id));
  console.log('first rows empty:', (await rows()).slice(0, 5).join(' | '));
  await page.keyboard.type('neu ord'); await wait();
  console.log('neu ord:', (await rows()).slice(0, 4).join(' | '));
  await shot('r7-palette-search');
  await page.keyboard.press('Enter'); await wait();
  console.log('after Enter: palette', await page.locator('.pal').count(), 'window', await page.locator('.win').getAttribute('aria-label').catch(() => null));
  await page.keyboard.press('Escape'); await wait();

  await page.keyboard.press('Control+k'); await wait();
  console.log('heads:', (await page.$$eval('.pal-list .head', hs => hs.map(h => h.textContent))).join(' | '), 'first:', (await rows())[0]);
  await shot('r7-palette-recent');
  await page.keyboard.type('sort'); await wait();
  console.log('sort:', (await page.$$eval('.pal-row', rs => rs.map(r => r.textContent.trim()))).slice(0, 5).join(' | '));
  await page.keyboard.press('Control+a'); await page.keyboard.type('lösch'); await wait();
  console.log('lösch:', (await rows()).join(' | '));
  await page.keyboard.press('Control+a'); await page.keyboard.type('xyzq'); await wait();
  console.log('no match:', await page.locator('.pal-none').textContent());
  await page.keyboard.press('Control+a'); await page.keyboard.type('vers dat'); await wait();
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('ArrowUp'); await page.keyboard.press('ArrowDown');
  console.log('vers dat, second picked:', await page.locator('.pal-row.cur .t').textContent());
  await page.keyboard.press('Escape'); await wait();
  console.log('after Escape:', await page.locator('.pal').count());

  await page.click('[data-menu="5"]'); await wait();
  console.log('Hilfe menu:', (await page.$$eval('#menu button', bs => bs.map(b => b.textContent.trim()))).join(' | '));
  await shot('r7-help-menu');
  await page.click('#menu button:first-child'); await wait();
  console.log('palette from menu:', await page.locator('.pal').count());
  await page.keyboard.press('Escape'); await wait();
  await page.keyboard.press('F1'); await wait();
  console.log('shortcut window has Strg+K:', await page.locator('.kbd-grid div', { hasText: 'Befehl suchen' }).textContent());
  await page.keyboard.press('Escape'); await wait();

  const tabs = await page.evaluate(() => CC.orderedTabs().map(t => t.id));
  for (const width of [1600, 1200]) {
    await page.setViewportSize({ width, height: 950 });
    for (const id of tabs) {
      await page.evaluate((t) => CC.go('settings', t), id); await wait(250);
      const m = await page.evaluate(() => {
        const cards = [...document.querySelectorAll('#settings-col > *')].map(c => c.getBoundingClientRect());
        const lefts = [...new Set(cards.map(r => Math.round(r.left)))];
        const bottoms = lefts.map(l => Math.max(...cards.filter(r => Math.round(r.left) === l).map(r => r.bottom)));
        const split = cards.some(c => c.height > document.querySelector('#settings-col').getBoundingClientRect().height);
        return `${cards.length} cards, ${lefts.length} col, width ${Math.round(cards[0].width)}, bottoms ${bottoms.map(Math.round).join('/')}${split ? ', split!' : ''}`;
      });
      console.log(`${width} ${id}: ${m}`);
      await shot(`r7-settings-${width}-${id}`);
    }
  }
};
