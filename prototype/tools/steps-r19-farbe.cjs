// Round 19, colours: the picker with its field, hue strip, hex value and proposed name, and the glyphs on menu choices.
module.exports = async (page, shot) => {
  const wait = (ms = 300) => page.waitForTimeout(ms);
  await page.setViewportSize({ width: 1600, height: 950 });
  await wait(600);
  const state = () => page.evaluate(() => `${$('.cp-hex').value} ${$('.cp-name').textContent}`);

  await page.click('.panel.active [data-labelsmenu]'); await page.click('#menu .hedit >> nth=0'); await wait();
  await page.click('[data-colpick="new"]'); await wait();
  console.log('picker parts:', await page.evaluate(() => ['.cp-sv', '.cp-hue', '.cp-hex', '.cp-name', '.cp-ok'].map(s => !!$(s)).join(' ')), 'swatch grid gone:', await page.evaluate(() => !$('.cp-grid')));
  console.log('start:', await state());

  const sv = await page.locator('.cp-sv').boundingBox(), hue = await page.locator('.cp-hue').boundingBox();
  await page.mouse.click(hue.x + hue.width * 0.62, hue.y + hue.height / 2); await wait();
  console.log('hue click:', await state());
  await page.mouse.move(sv.x + sv.width * 0.2, sv.y + sv.height * 0.2);
  await page.mouse.down();
  await page.mouse.move(sv.x + sv.width * 0.85, sv.y + sv.height * 0.35, { steps: 6 });
  await page.mouse.move(sv.x + sv.width * 1.4, sv.y - 30, { steps: 4 });
  await page.mouse.up(); await wait();
  console.log('drag past the edge:', await state());
  await page.focus('.cp-sv'); await page.keyboard.press('ArrowDown'); await page.keyboard.press('Shift+ArrowDown'); await wait();
  console.log('keys on field:', await state());
  await page.focus('.cp-hue'); await page.keyboard.press('Shift+ArrowRight'); await wait();
  console.log('keys on strip:', await state());
  await shot('r19-farbe-1-dark');

  await page.fill('.cp-hex', '#5e1224'); await wait();
  console.log('hex typed:', await state());
  await page.click('.cp-ok'); await wait();
  console.log('field gets the name:', await page.inputValue('#mk-new'));
  for (const hex of ['#1b6b73', '#98e2c6', '#d2a91f', '#c8a2d6', '#383e42', '#fbb57c']) {
    await page.click('[data-colpick="new"]'); await page.fill('.cp-hex', hex); await page.keyboard.press('Enter'); await wait(150);
    console.log(`  ${hex} ->`, await page.inputValue('#mk-new'));
  }
  await page.fill('#mk-new', 'Meine Farbe');
  await page.click('[data-colpick="new"]'); await page.fill('.cp-hex', '#ff4fa3'); await page.keyboard.press('Enter'); await wait();
  console.log('own name stays:', await page.inputValue('#mk-new'));
  await page.click('[data-coladd]'); await wait();

  await page.click('[data-colpick="0"]'); await wait();
  await page.keyboard.press('Escape'); await wait();
  console.log('escape closes picker, window stays:', await page.evaluate(() => `${!$('.cpick')} ${!!$('.win')}`));
  await page.click('[data-colpick="0"]'); await page.fill('.cp-hex', '#7b1f2e'); await page.click('.cp-ok'); await wait();
  console.log('row 0 colour:', await page.evaluate(() => $('[data-colpick="0"]').style.getPropertyValue('--c')));

  await page.evaluate(() => document.documentElement.dataset.theme = 'light'); await wait();
  await page.click('[data-colpick="new"]'); await page.fill('.cp-hex', '#2a8ff7'); await wait();
  await shot('r19-farbe-2-light');
  await page.keyboard.press('Escape');
  await page.click('.win-foot .btn.primary'); await wait();
  console.log('colours:', await page.evaluate(() => LABEL_COLORS.map(c => `${c[1]} ${c[2]}`).join(', ')));

  await page.evaluate(() => document.documentElement.dataset.theme = 'dark');
  await page.click('.panel.active [data-viewmenu]'); await wait();
  console.log('view menu:', await page.evaluate(() => $$('#menu button').map(b => `${b.innerText.trim()}${b.querySelector('svg') ? '+g' : ''}${b.classList.contains('on') ? '(on)' : ''}${b.querySelector('.cbox') ? '[box]' : ''}`).join(' | ')));
  await shot('r19-farbe-3-viewmenu');
  await page.keyboard.press('Escape'); await page.evaluate(() => closeMenu());
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); CC.run('selectAll'); CC.run('copy'); }); await wait();
  await page.click('#clash'); await wait();
  console.log('clash menu:', await page.evaluate(() => $$('#menu :is(button, [role=option])').map(b => `${b.innerText.trim()}${b.querySelector('svg') ? '+g' : ''}${b.classList.contains('on') ? '(on)' : ''}`).join(' | ')));
  await shot('r19-farbe-4-clash');
};
