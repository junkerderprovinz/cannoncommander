// Round 4: Explorer columns with search, shortcut menus, path fields follow the folder, pack window, Krusader extras.
module.exports = async (page, shot) => {
  const wait = (ms = 400) => page.waitForTimeout(ms);
  const dots = () => page.evaluate(() => [...document.querySelectorAll('.pwdot:not(.static)')].filter(d => d.getBoundingClientRect().width > 0).length);
  await wait(800);
  console.log('dots on files:', await dots());
  await shot('r4-main');

  // Every column renders for every kind of file without throwing.
  const colErr = await page.evaluate(() => {
    const bad = [];
    for (const dir of ['/mnt/user/downloads', '/mnt/user/fotos/2026', '/mnt/user/musik', '/mnt/user/isos', '/mnt/user/dokumente', '/mnt/user']) {
      const n = FS.nodeAt(dir);
      if (!n) continue;
      for (const k of FS.kids(n)) for (const c of COLUMNS) {
        try { const v = c.value ? c.value({ ...k, kind: FS.kindOf(k) }, dir) : ''; if (v === undefined) bad.push(`${c.id} undefined for ${k.name}`); } catch (e) { bad.push(`${c.id} on ${k.name}: ${e.message}`); }
      }
    }
    return { count: COLUMNS.length, bad: bad.slice(0, 10) };
  });
  console.log('columns:', colErr.count, 'problems:', colErr.bad.length ? colErr.bad.join('; ') : 'none');

  await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); Columns.set(['name', 'size', 'camera', 'make', 'iso', 'aperture', 'place']); });
  await wait(300);
  await shot('r4-photo-cols');
  await page.evaluate(() => { Files.go('/mnt/user/musik', 0); Columns.set(['name', 'size', 'title', 'artist', 'track', 'abitrate', 'genre']); });
  await wait(300);
  await shot('r4-music-cols');
  await page.evaluate(() => Columns.set(DEFAULT_COLUMNS));

  await page.click('.panel.active .lhead', { button: 'right' }); await wait(250);
  console.log('header menu items:', await page.locator('#menu [role="menuitemcheckbox"]').count());
  await shot('r4-colmenu');
  await page.keyboard.press('Escape');
  await page.evaluate(() => CC.run('columns')); await wait(300);
  await page.fill('#col-q', 'codec'); await wait(200);
  console.log('rows for "codec":', await page.locator('.col-grid .frow:visible').count());
  await shot('r4-colsearch');
  await page.fill('#col-q', 'fotos'); await wait(200);
  console.log('rows for "fotos":', await page.locator('.col-grid .frow:visible').count());
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });

  // Shortcuts: context menu, rename, remove with undo, add from the tree and the list.
  const before = await page.evaluate(() => DATA.bookmarks.length);
  await page.click('#shortcuts [data-bm="0"]', { button: 'right' }); await wait(250);
  console.log('shortcut menu:', (await page.locator('#menu [role="menuitem"]').allTextContents()).join(' | '));
  await shot('r4-bm-menu');
  await page.click('#menu >> text=Verknüpfung entfernen'); await wait(250);
  console.log('after remove:', await page.evaluate(() => DATA.bookmarks.length), 'was', before);
  await page.click('.toast >> text=Rückgängig').catch(() => console.log('no undo button'));
  await wait(250);
  console.log('after undo:', await page.evaluate(() => DATA.bookmarks.length));
  await page.click('#tree [data-go="/mnt/disks"], #tree [data-go="/boot"]', { button: 'right' }).catch(() => {});
  await wait(250);
  await page.click('#menu >> text=Als Verknüpfung anlegen').catch(() => console.log('no add on tree'));
  await wait(250);
  console.log('after tree add:', await page.evaluate(() => DATA.bookmarks.map(b => b.name).join(', ')));
  await page.evaluate(() => { Files.go('/mnt/user', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'isos'); t.marked = new Set(); CC.render(); });
  await page.click('.panel.active .row.cursor', { button: 'right' }); await wait(250);
  await page.click('#menu >> text=Als Verknüpfung anlegen').catch(() => console.log('no add on row'));
  await wait(250);
  console.log('after row add:', await page.evaluate(() => DATA.bookmarks.map(b => b.name).join(', ')));
  await shot('r4-bm-added');

  // Path fields start in the folder you are in.
  await page.evaluate(() => { Files.go('/mnt/user/fotos', 0); Files.active = 0; CC.run('duplicates'); });
  await wait(300);
  console.log('duplicates path:', await page.inputValue('#u-in'));
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); CC.run('search'); });
  await wait(300);
  console.log('search path:', await page.inputValue('#s-in'));
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); Files.go('/mnt/user/musik', 0); CC.run('search'); });
  await wait(300);
  console.log('search path after moving:', await page.inputValue('#s-in'));
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });

  // Pack window.
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => !k.dir); CC.render(); CC.run('pack'); });
  await wait(300);
  await shot('r4-pack');
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });

  // Krusader extras.
  await page.evaluate(() => { CC.setPref('showExt', false); });
  await wait(200);
  await shot('r4-noext');
  await page.evaluate(() => { CC.setPref('showExt', true); CC.prefs.dual = true; CC.setPref('syncBrowse', true); Files.go('/mnt/user', 0); Files.go('/mnt/user', 1); Files.active = 0; Files.go('/mnt/user/fotos', 0); });
  await wait(200);
  console.log('sync browse right panel:', await page.evaluate(() => Files.T(1).path));
  await page.evaluate(() => CC.setPref('syncBrowse', false));
  await page.keyboard.press('Alt+ArrowDown'); await wait(250);
  console.log('history menu items:', await page.locator('#menu [role="menuitem"]').count());
  await page.keyboard.press('Escape');
  await page.evaluate(() => CC.run('profiles')); await wait(300);
  await shot('r4-profiles');
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); CC.go('settings'); });
  await wait(300);
  await shot('r4-settings');
};
