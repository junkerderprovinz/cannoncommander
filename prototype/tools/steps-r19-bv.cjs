// The colours card, the three hidden modes and the Info tab, built after BombVault.
module.exports = async (page, shot) => {
  const wait = (ms = 300) => page.waitForTimeout(ms);
  await page.setViewportSize({ width: 1600, height: 1250 });
  await wait(500);
  await page.evaluate(() => { CC.setPref('shape', 'soft'); CC.setPref('motion', 'subtle'); CC.setPref('rainbow', false); CC.setPref('disco', false); CC.go('settings', 'look'); });
  await wait(500);
  const rows = () => page.evaluate(() => $$('.card').find(c => c.querySelector('.notch')?.textContent === 'Farben').innerText.replace(/\s+/g, ' '));
  console.log('colours, rainbow off:', await rows());
  await shot('r19-bv-1-colours-off');

  // Blatt: five more clicks on Eckig once it is chosen.
  await page.click('#a-shape [data-v="square"]'); await wait();
  for (let i = 0; i < 5; i++) { await page.click('#a-shape [data-v="square"]'); await wait(80); }
  console.log('leaf:', await page.evaluate(() => `${CC.prefs.shape} ${document.documentElement.dataset.shape} offered ${!!$('#a-shape [data-v="leaf"]')}`));
  // Sturm: five more clicks on Wild once it is chosen.
  await page.click('#a-motion [data-v="wild"]'); await wait();
  for (let i = 0; i < 5; i++) { await page.click('#a-motion [data-v="wild"]'); await wait(80); }
  console.log('storm:', await page.evaluate(() => `${CC.prefs.motion} ${document.documentElement.dataset.motion} offered ${!!$('#a-motion [data-v="storm"]')}`));
  await shot('r19-bv-2-leaf-storm');

  // Disco: five turn-ons of the rainbow, each within three seconds.
  for (let i = 0; i < 5; i++) {
    await page.click('#a-rainbow'); await wait(120);
    if (i < 4) { await page.click('#a-rainbow'); await wait(120); }
  }
  console.log('disco offered:', await page.evaluate(() => !!$('[data-pref="disco"]')), 'rainbow:', await page.evaluate(() => CC.prefs.rainbow));
  console.log('colours, rainbow on:', await rows());
  await page.click('[data-pref="disco"]'); await wait(1200);
  const rb = () => page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--rb-0').trim());
  const a = await rb(); await wait(1500); const b = await rb();
  console.log('disco walks:', a, '->', b, a !== b, 'attr', await page.evaluate(() => document.documentElement.hasAttribute('data-disco')));
  await shot('r19-bv-3-colours-on');
  await page.click('[data-pref="disco"]'); await wait(300);
  console.log('disco off, rest palette:', await rb());

  // Rotation draws an offset; the palette reset and an edited palette slot.
  await page.click('#a-rotate'); await wait();
  console.log('rotation seed:', await page.evaluate(() => CC.prefs.rotateSeed), '--rb-0', await rb(), 'data-rotate', await page.evaluate(() => document.documentElement.hasAttribute('data-rotate')));
  await page.click('[data-palette="0"]'); await wait();
  console.log('palette picker open:', await page.evaluate(() => !!$('.a-cpop')));
  await page.fill('.a-cpop input', '#123456'); await wait();
  await page.keyboard.press('Escape'); await wait();
  console.log('palette 0:', await page.evaluate(() => CC.prefs.palette[0]), 'reset enabled:', await page.evaluate(() => !$('#a-palette-reset').disabled));
  await page.click('#a-palette-reset'); await wait();
  console.log('palette after reset:', await page.evaluate(() => CC.prefs.palette.join(' ')));
  console.log('accent row dimmed while rainbow on:', await page.evaluate(() => $('.a-accent').classList.contains('dim')), 'presets disabled:', await page.evaluate(() => $('[data-preset="0"]').disabled));

  // Accent: rainbow off, a preset is chosen and its picker opens.
  await page.click('#a-rainbow'); await wait();
  await page.click('[data-preset="1"]'); await wait();
  console.log('accent:', await page.evaluate(() => CC.prefs.accent), 'picker:', await page.evaluate(() => !!$('.a-cpop')));
  await page.keyboard.press('Escape'); await wait();
  await page.click('#a-accent-reset'); await wait();
  console.log('accent after reset:', await page.evaluate(() => CC.prefs.accent));

  // Info tab in both looks, and the two giving windows.
  await page.evaluate(() => { CC.setPref('shape', 'soft'); CC.setPref('motion', 'subtle'); CC.go('settings', 'info'); }); await wait(500);
  console.log('info cards:', await page.evaluate(() => $$('.card .notch').map(n => n.textContent).join(' | ')));
  console.log('version row:', await page.evaluate(() => $$('.card')[0].innerText.replace(/\s+/g, ' ')));
  console.log('quantum gone:', await page.evaluate(() => !document.body.innerText.includes('Quantum')));
  await shot('r19-bv-4-info-dark');
  await page.click('#a-update'); await wait(1200);
  console.log('update:', await page.evaluate(() => $('#a-update').innerText));
  await page.evaluate(() => { CC.setPref('theme', 'light'); CC.go('settings', 'info'); }); await wait(400);
  await shot('r19-bv-5-info-light');
  await page.click('#a-paypal'); await wait();
  await shot('r19-bv-6-paypal');
  await page.evaluate(() => CC.closeWin());
  await page.click('#a-crypto'); await wait();
  await page.click('#cr-coin [data-v="xrp"]'); await wait();
  console.log('crypto:', await page.evaluate(() => `${$('#cr-addr').textContent} | ${$('#cr-note').textContent}`));
  await shot('r19-bv-7-crypto');
  await page.evaluate(() => { CC.closeWin(); CC.go('settings', 'look'); }); await wait(400);
  await shot('r19-bv-8-colours-light');
  await page.evaluate(() => { CC.setPref('theme', 'dark'); CC.setPref('rainbow', false); });
};
