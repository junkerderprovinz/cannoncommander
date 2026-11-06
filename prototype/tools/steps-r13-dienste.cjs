// Round 13, services: job queue, saved searches, rename metadata, fetch from an address, collection links,
// link options, touch selection, install as an app, jump keys and the live region.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  const closeAll = () => page.evaluate(() => { closeMenu(); while (winStack.length) CC.closeWin(); });
  const smartMenu = async () => { await page.evaluate(() => { closeMenu(); Object.assign(Files.T(), { qf: false, filter: '' }); CC.render(); }); await page.click('.panel.active [data-run="quickFilter"]'); await wait(); };
  const live = () => page.evaluate(() => $('#cc-live').textContent);
  await wait(600);

  // 1. Jobs: three copies in a row, one runs and two wait; reorder with the arrows and by dragging; then side by side.
  await page.evaluate(async () => {
    CC.setPref('jobMode', 'serial');
    const kids = FS.kids(FS.nodeAt('/mnt/user/backups')), pick = (...names) => names.map(n => kids.find(k => k.name === n));
    for (const items of [pick('appdata-2026-10-24.tar.zst', 'appdata-2026-10-31.tar.zst'), pick('vm-win11-2026-10-01.img'), pick('flash-2026-10-31.zip')]) await Files.transfer('copy', items, '/mnt/user/dokumente', '/mnt/user/backups');
  });
  await wait(900);
  await page.evaluate(() => CC.run('jobs')); await wait(400);
  const jobs = () => page.evaluate(() => Jobs.running().map(j => `${j.label.split(': ')[1].split(' nach')[0]} [${j.state}]`).join(' | '));
  console.log('1 serial:', await jobs());
  console.log('1 first row:', await page.evaluate(() => $('#jobs-body .job').innerText.replace(/\s+/g, ' ')));
  await shot('r13-1-jobs-serial');
  await page.click('#jobs-body .job:nth-child(3) [data-jact="up"]'); await wait();
  console.log('1 after up:', await jobs());
  await page.dragAndDrop('#jobs-body .job:nth-child(3)', '#jobs-body .job:nth-child(1)', { targetPosition: { x: 40, y: 4 } }); await wait();
  console.log('1 after drag:', await jobs());
  await page.click('#jobs-mode [data-v="parallel"]'); await page.mouse.move(10, 10); await wait(600);
  console.log('1 parallel:', await jobs());
  await shot('r13-1-jobs-parallel');
  await page.evaluate(() => { Jobs.running().forEach(Jobs.cancel); Jobs.list = []; Jobs.changed(); CC.setPref('jobMode', 'serial'); });
  await closeAll();

  // 2. Saved searches: save *.mkv from Alt+F7, open it from the sidebar, rename and remove it by right click.
  await page.evaluate(() => CC.run('search')); await wait();
  await page.fill('#s-in', '/mnt/user');
  await page.fill('#s-name', '*.mkv');
  await page.waitForSelector('.bk-hits:not([hidden]) #s-save');
  await page.click('#s-save');
  await page.waitForSelector('#sm-name', { timeout: 5000 }).catch(async () => console.log('save window missing, windows:', await page.evaluate(() => $$('.win').map(w => w.getAttribute('aria-label')).join(' | '))));
  await page.fill('#sm-name', 'Alle Filme');
  await page.click('.scrim:last-of-type .win-foot .btn.primary'); await wait();
  await closeAll(); await wait();
  await smartMenu();
  console.log('2 search menu:', await page.evaluate(() => $$('.qdrop .qrow').map(r => r.innerText.replace(/\s+/g, ' ')).join(' | ')));
  await page.click('.qdrop .qrow:has-text("Alle Filme")'); await wait();
  console.log('2 panel:', await page.evaluate(() => `${Files.entries().length} hits, bar: ${$('.panel.active .smart-bar')?.innerText}`));
  await shot('r13-2-smart');
  await smartMenu();
  await page.click('.qdrop .qrow:has-text("Alle Filme")', { button: 'right' }); await wait();
  await shot('r13-2-smart-menu');
  await page.click('#menu button:has-text("Umbenennen")'); await wait();
  await page.fill('#sm-name', 'Filme überall'); await page.keyboard.press('Enter'); await wait();
  await smartMenu();
  await page.click('.qdrop .qrow:has-text("Filme überall")', { button: 'right' }); await wait();
  await page.click('#menu button:has-text("Entfernen")'); await wait();
  console.log('2 after rename and remove:', await page.evaluate(() => (CC.smart.rows().match(/<span>[^<]*/g) || []).map(s => s.slice(6)).join(' | ')), 'panel smart:', await page.evaluate(() => Files.T().smart));

  // 3. Rename with metadata: the photo preset, the series preset applied, an own preset saved.
  await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); CC.run('selectAll'); CC.run('multiRename'); }); await wait();
  await page.click('#m-preset'); await wait(200);
  await page.click('#menu :is(button, [role=option]):has-text("Fotos nach Datum")'); await wait();
  console.log('3 photos:', await page.evaluate(() => $$('#m-list .bk-mrow').slice(0, 2).map(r => r.innerText.replace(/\s+/g, ' ')).join(' | ')));
  await shot('r13-3-rename-photos');
  await closeAll();
  await page.evaluate(() => { Files.go('/mnt/user/serien/Pioneer One/Staffel 1', 0); CC.run('selectAll'); CC.run('multiRename'); }); await wait();
  await page.click('#m-preset'); await wait(200);
  await page.click('#menu :is(button, [role=option]):has-text("Serie S01E02")'); await wait();
  await page.fill('#m-name', 'Pioneer One S[S2]E[F2]'); await wait();
  await page.click('#m-psave'); await wait();
  await page.fill('#m-pname', 'Pioneer One'); await page.click('.scrim:last-of-type .win-foot .btn.primary'); await wait();
  console.log('3 presets:', await page.evaluate(() => JSON.parse($('#m-preset').dataset.dd).map(o => o[1]).join(', ')), 'picked:', await page.evaluate(() => $('#m-preset').innerText));
  await shot('r13-3-rename-series');
  await page.click('.win-foot .btn.primary:has-text("Umbenennen")'); await wait();
  console.log('3 series renamed:', await page.evaluate(() => FS.kids(FS.nodeAt('/mnt/user/serien/Pioneer One/Staffel 1')).map(k => k.name).slice(0, 3).join(', ')));
  await page.evaluate(() => { Files.go('/mnt/user/musik/Kevin MacLeod', 0); CC.run('selectAll'); CC.run('multiRename'); }); await wait();
  await page.fill('#m-name', '[I] - [T] [R]'); await wait();
  console.log('3 music, missing hint:', await page.evaluate(() => $('#m-list .bk-mrow').innerText.replace(/\s+/g, ' ')));
  await closeAll();

  // 4. Fetch from an address into the open folder.
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); CC.run('fetchUrl'); }); await wait();
  await page.fill('#fu-url', 'https://releases.ubuntu.com/24.04/ubuntu-24.04.3-live-server-amd64.iso'); await wait(200);
  console.log('4 suggested name:', await page.inputValue('#fu-name'));
  await shot('r13-4-fetch');
  await page.click('.win-foot .btn.primary'); await wait(1200);
  await page.evaluate(() => CC.run('jobs')); await wait(300);
  console.log('4 job:', await page.evaluate(() => $('#jobs-body .job')?.innerText.replace(/\s+/g, ' ')));
  await shot('r13-4-fetch-job');
  await closeAll();
  await page.waitForFunction(() => FS.kids(FS.nodeAt('/mnt/user/downloads')).some(k => k.name === 'ubuntu-24.04.3-live-server-amd64.iso'), null, { timeout: 30000 }).catch(() => {});
  console.log('4 landed:', await page.evaluate(() => FS.kids(FS.nodeAt('/mnt/user/downloads')).some(k => k.name === 'ubuntu-24.04.3-live-server-amd64.iso')));
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); CC.run('fetchUrl'); }); await wait();
  await page.fill('#fu-url', 'kein link'); await page.click('.win-foot .btn.primary'); await wait();
  console.log('4 bad address keeps the window:', await page.locator('.scrim .win').count());
  await closeAll();

  // 5. Collection: posters from three film folders found by the panel search, plus one file from elsewhere, one link.
  await page.evaluate(() => { Files.go('/mnt/user/filme', 0); Files.active = 0; Object.assign(Files.T(), { filter: 'poster', qf: true }); CC.render(); CC.run('selectAll'); }); await wait();
  console.log('5 share from search result:', await page.evaluate(() => CC.blocked(CC.actions.share) || 'allowed'));
  await page.evaluate(() => CC.run('collect')); await wait();
  await page.evaluate(() => { Files.go('/mnt/user/dokumente', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'Mietvertrag.pdf'); CC.render(); CC.run('collect'); }); await wait();
  console.log('5 sidebar row:', await page.evaluate(() => $('[data-run="collection"]')?.innerText.replace(/\s+/g, ' ')));
  await shot('r13-5-basket-side');
  await page.evaluate(() => CC.run('collection')); await wait();
  await shot('r13-5-basket');
  await page.click('.win-foot .btn.primary'); await wait();
  await shot('r13-5-collection-share');
  await page.click('.win-foot .btn.primary'); await wait();
  console.log('5 visitor sees:', await page.evaluate(() => $$('.cs-vrow span:first-of-type').map(s => s.textContent).join(', ')));
  await shot('r13-5-collection-done');
  await closeAll();
  console.log('5 basket after share:', await page.evaluate(() => CC.actions.collection.label()), 'shares page:', await page.evaluate(() => { CC.go('settings', 'shares'); return $('.a-row .t').textContent; }));
  await shot('r13-5-shares-page');
  await page.evaluate(() => CC.go('files'));

  // 6. Link options: limit, own address, rolling a new one, notice on open.
  await page.evaluate(() => { Files.go('/mnt/user/fotos', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'Urlaub'); CC.render(); CC.run('share'); }); await wait();
  const slug1 = await page.inputValue('#cs-slug');
  await page.click('#cs-dice'); await wait(150);
  console.log('6 rolled:', slug1, '->', await page.inputValue('#cs-slug'));
  await page.click('#cs-existing [data-reroll]'); await wait(200);
  console.log('6 existing rerolled:', await page.evaluate(() => DATA.shares.find(s => s.path === '/mnt/user/fotos/Urlaub').slug));
  await page.fill('#cs-slug', 'x'); await page.click('.win-foot .btn.primary'); await wait();
  console.log('6 short address keeps the window:', await page.locator('.scrim .win').count());
  await page.fill('#cs-slug', 'urlaub-2026');
  await page.click('#cs-limit [data-v="5"]');
  await page.click('#cs-notify'); await wait(150);
  await shot('r13-6-share-options');
  await page.click('.win-foot .btn.primary'); await wait(500);
  console.log('6 link:', await page.evaluate(() => { const s = DATA.shares[0]; return `${s.slug}, limit ${s.limit}, notify ${s.notify}`; }));
  await shot('r13-6-share-done');
  await page.waitForTimeout(6300);
  console.log('6 notice:', await page.evaluate(() => $$('.toast').map(t => t.innerText.replace(/\s+/g, ' ')).join(' | ')));
  await shot('r13-6-share-notice');
  await closeAll();

  // 7. Touch: a long press starts the selection mode on a phone-sized screen.
  await page.setViewportSize({ width: 390, height: 844 }); await wait(300);
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); Files.active = 0; CC.render(); }); await wait();
  const press = (n) => page.evaluate((n) => {
    const r = $$('.panel.active .list .row')[n], b = r.getBoundingClientRect(), at = { bubbles: true, pointerType: 'touch', clientX: b.left + 30, clientY: b.top + 10, isPrimary: true };
    r.dispatchEvent(new PointerEvent('pointerdown', at));
    return new Promise(res => setTimeout(() => { r.dispatchEvent(new PointerEvent('pointerup', at)); r.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: at.clientX, clientY: at.clientY })); res(); }, 650));
  }, n);
  await press(1); await wait();
  console.log('7 mode on:', await page.evaluate(() => $('#app').classList.contains('touch-sel')), 'marked:', await page.evaluate(() => [...Files.T().marked].join(', ')));
  await page.click('.panel.active .list .row:nth-of-type(4)'); await wait();
  await page.click('.panel.active .list .row:nth-of-type(5)'); await wait();
  console.log('7 after taps:', await page.evaluate(() => [...Files.T().marked].join(', ')), 'bar:', await page.evaluate(() => $('.touchbar').innerText.replace(/\s+/g, ' ')));
  await shot('r13-7-touch-390');
  await page.click('.touchbar [data-run="copy"]'); await wait();
  console.log('7 copy window:', await page.evaluate(() => $('.scrim .win .notch')?.textContent));
  await closeAll();
  await page.click('[data-tsel-done]'); await wait();
  console.log('7 mode off:', !(await page.evaluate(() => $('#app').classList.contains('touch-sel'))));
  await page.setViewportSize({ width: 1600, height: 950 }); await wait(300);

  // 8. Install: manifest with share target, icon from the folder glyph, the help entry.
  console.log('8 manifest:', await page.evaluate(async () => {
    const m = await (await fetch($('link[rel="manifest"]').href)).json();
    return `${m.name}, ${m.display}, theme ${m.theme_color}, icons ${m.icons.length}, share_target ${m.share_target.method} ${Object.keys(m.share_target.params).join('/')}`;
  }));
  console.log('8 help menu:', await page.evaluate(() => MENUS.find(m => m[0] === 'Hilfe')[1].join(', ')));
  await page.evaluate(() => CC.run('installApp')); await wait();
  await shot('r13-8-install');
  await closeAll();

  // 9. Jump keys and announcements.
  await page.evaluate(() => { CC.prefs.dual = true; CC.render(); $('.panel.active .list').focus(); }); await wait();
  await page.keyboard.press('Alt+1'); await wait();
  await page.keyboard.press('ArrowDown'); await wait(100);
  console.log('9 Alt+1 focus:', await page.evaluate(() => document.activeElement.innerText.split('\n')[0]), '| live:', await live());
  await page.keyboard.press('Alt+3'); await wait();
  console.log('9 Alt+3:', await page.evaluate(() => `active ${Files.active}, focus ${document.activeElement.getAttribute('aria-label')}`), '| live:', await live());
  await page.keyboard.press('Alt+2'); await wait();
  await page.keyboard.press('Insert'); await page.keyboard.press('Insert'); await wait(300);
  console.log('9 after marking:', await live());
  await page.keyboard.press('Alt+4'); await wait();
  console.log('9 Alt+4:', await page.evaluate(() => $('.scrim .win .notch')?.textContent), '| live:', await live());
  await closeAll();
  await page.evaluate(async () => { const n = FS.kids(FS.nodeAt('/mnt/user/downloads')).find(k => k.name === 'setup-notes.md'); await Files.transfer('copy', [n], '/mnt/user/dokumente', '/mnt/user/downloads'); });
  await page.waitForTimeout(1500);
  console.log('9 job done:', await live());
  await page.keyboard.press('F1'); await wait();
  console.log('9 shortcuts window:', await page.evaluate(() => $$('.kb-row').filter(d => /Alt\+[1-4]/.test(d.innerText)).map(d => d.innerText.replace(/\s+/g, ' ')).join(' | ')));
  await page.evaluate(() => $$('.kb-row').find(d => /Alt\+4/.test(d.innerText)).scrollIntoView({ block: 'center' })); await wait(200);
  await shot('r13-9-shortcuts');
  await closeAll();
};
