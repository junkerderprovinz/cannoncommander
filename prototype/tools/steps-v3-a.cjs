// Everyday handling: jump by typing, undo with Strg+Z and its list, ranked folders, the folder in the address,
// files from the clipboard, duplicating and a new folder around the selection.
module.exports = async (page, shot) => {
  const wait = (ms = 300) => page.waitForTimeout(ms);
  const closeAll = () => page.evaluate(() => { closeMenu(); while (winStack.length) CC.closeWin(); });
  const cursor = () => page.evaluate(() => Files.entries()[Files.T().cursor]?.name);
  const names = () => page.evaluate(() => Files.entries().map(k => k.name).join(', '));
  await page.setViewportSize({ width: 1600, height: 950 });
  await wait(600);

  // Typing jumps, arrows step through the hits, Escape ends.
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); $('.panel.active .list').focus(); }); await wait();
  await page.keyboard.type('ub'); await wait();
  console.log('jump ub:', await cursor(), '| pill:', await page.evaluate(() => $('.tjump')?.innerText.replace(/\s+/g, ' ')));
  await shot('v3a-1-jump');
  await page.keyboard.press('ArrowDown'); await wait();
  console.log('next hit:', await cursor());
  await page.keyboard.type('zz'); await wait();
  console.log('miss:', await page.evaluate(() => $('.tjump')?.classList.contains('miss')), await cursor());
  await page.keyboard.press('Escape'); await wait();
  console.log('after Esc pill:', await page.evaluate(() => !!$('.tjump')));
  await page.keyboard.press('Space'); await wait();
  console.log('space still quick look:', await page.evaluate(() => !!$('.ql, .quicklook, [aria-label*="Schnellansicht"]')));
  await page.keyboard.press('Escape'); await closeAll();

  // Duplicate and gather into a new folder, each undone with Strg+Z.
  await page.evaluate(() => { Files.go('/mnt/user/dokumente', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'notizen.txt'); CC.render(); $('.panel.active .list').focus(); }); await wait();
  await page.keyboard.press('Control+d'); await wait();
  await page.keyboard.press('Control+d'); await wait();
  console.log('duplicates:', (await names()).split(', ').filter(n => n.startsWith('notizen')).join(', '));
  await page.evaluate(() => { const t = Files.T(); t.marked = new Set(['notizen Kopie.txt', 'notizen Kopie 2.txt']); CC.render(); $('.panel.active .list').focus(); });
  await page.keyboard.press('Shift+F7'); await wait();
  console.log('gather window:', await page.evaluate(() => $('.win .notch')?.textContent), await page.inputValue('#fws-name'));
  await page.fill('#fws-name', 'Kopien'); await page.keyboard.press('Enter'); await wait();
  console.log('after gather:', (await names()).split(', ').filter(n => /notizen|Kopien/.test(n)).join(', '), '| inside:', await page.evaluate(() => FS.kids(FS.nodeAt('/mnt/user/dokumente/Kopien')).map(k => k.name).join(', ')));
  await shot('v3a-2-gather');

  // Strg+Z takes back the newest step, the list takes back any of the last ten.
  await page.evaluate(() => $('.panel.active .list').focus());
  await page.keyboard.press('Control+z'); await wait();
  console.log('undo gather:', (await names()).split(', ').filter(n => /notizen|Kopien/.test(n)).join(', '));
  await page.evaluate(() => CC.run('undoList')); await wait();
  console.log('history menu:', await page.evaluate(() => $$('#menu > *').map(b => b.innerText.replace(/\s+/g, ' ').trim()).join(' | ')));
  await shot('v3a-3-history');
  await page.click('#menu button >> nth=0'); await wait();
  console.log('after list undo:', (await names()).split(', ').filter(n => /notizen/.test(n)).join(', '));
  await page.evaluate(() => { const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'notizen.txt'); CC.render(); CC.run('rename'); }); await wait();
  await page.fill('#nm', 'gedanken.txt'); await page.keyboard.press('Enter'); await wait();
  await page.evaluate(() => $('.panel.active .list').focus());
  await page.keyboard.press('Control+z'); await wait();
  console.log('rename undone:', (await names()).includes('notizen.txt'));
  await page.evaluate(() => { const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'notizen.txt'); CC.render(); CC.deleteItems(Files.sel(), Files.T().path); }); await wait();
  if (await page.isVisible('.xwin')) { await page.click('.xwin .win-foot .btn.primary'); await wait(); }
  console.log('deleted:', !(await names()).includes('notizen.txt'), '| history top:', await page.evaluate(() => CC.history.list[0]?.label));
  await page.evaluate(() => $('.panel.active .list').focus());
  await page.keyboard.press('Control+z'); await wait();
  console.log('delete undone by Strg+Z:', (await names()).includes('notizen.txt'), '| toast button now inert:', await page.evaluate(() => CC.history.list.every(x => !x.label.includes('Papierkorb'))));

  // A move is taken back as well, and the Unraid disks follow both ways.
  await page.evaluate(async () => { const k = FS.kids(FS.nodeAt('/mnt/user/dokumente')).find(n => n.name === 'aktuell.md'); await Files.transfer('move', [k], '/mnt/user/downloads', '/mnt/user/dokumente'); });
  await wait(2500);
  const where = () => page.evaluate(() => ['dokumente', 'downloads'].filter(d => FS.kids(FS.nodeAt('/mnt/user/' + d)).some(n => n.name === 'aktuell.md')).join(','));
  console.log('moved to:', await where());
  await page.evaluate(() => $('.panel.active .list').focus());
  await page.keyboard.press('Control+z'); await wait();
  await page.evaluate(() => CC.render()); await wait();
  console.log('move undone, back in:', await where());

  // Folders ranked by use: the command search and the folder history list them.
  for (const p of ['/mnt/user/filme', '/mnt/user/fotos', '/mnt/user/filme', '/mnt/user/filme', '/mnt/user/dokumente']) await page.evaluate((p) => Files.go(p, 0), p);
  console.log('top folders:', await page.evaluate(() => CC.topFolders(3).join(', ')));
  await page.keyboard.press('Control+k'); await wait();
  console.log('palette empty:', await page.evaluate(() => $$('.pal-list .head').map(h => h.textContent).join(' | ')));
  await page.keyboard.type('filme'); await wait();
  console.log('palette filme:', await page.evaluate(() => $$('.pal-row .t').slice(0, 3).map(r => r.textContent).join(' | ')));
  await shot('v3a-4-palette');
  await page.keyboard.press('Enter'); await wait();
  console.log('palette opened:', await page.evaluate(() => Files.T().path));
  await page.evaluate(() => CC.run('history')); await wait();
  console.log('history:', await page.evaluate(() => $$('#menu > *').slice(0, 6).map(b => b.innerText.trim()).join(' | ')));
  await closeAll();

  // The folder sits in the address; back and forward and windows keep working; a reload comes back.
  console.log('hash:', await page.evaluate(() => decodeURIComponent(location.hash)));
  await page.evaluate(() => Files.go('/mnt/user/serien', 0)); await wait();
  await page.goBack(); await wait();
  console.log('back:', await page.evaluate(() => `${Files.T().path} ${decodeURIComponent(location.hash)}`));
  await page.goForward(); await wait();
  console.log('forward:', await page.evaluate(() => `${Files.T().path} ${decodeURIComponent(location.hash)}`));
  await page.evaluate(() => CC.run('diskUsage', '/mnt/user')); await wait(400);
  await page.click('#du-view [data-v="tree"]'); await wait();
  await page.dblclick('#du-body .du-row.dir >> nth=0'); await wait();
  await page.goBack(); await wait();
  console.log('window back:', await page.evaluate(() => $('#du-crumbs').innerText.replace(/\s+/g, ' ')));
  await closeAll(); await wait(400);
  console.log('after window closed:', await page.evaluate(() => `${Files.T().path} ${decodeURIComponent(location.hash)}`));
  await page.goBack(); await wait();
  console.log('panel back after window:', await page.evaluate(() => Files.T().path));
  await page.evaluate(() => { location.hash = '#/mnt/user/musik'; }); await wait();
  console.log('typed hash:', await page.evaluate(() => Files.T().path));
  await page.reload(); await wait(900);
  console.log('after reload:', await page.evaluate(() => Files.T().path));
  await page.evaluate(() => { Files.go('/mnt/user', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'fotos'); CC.render(); });
  await page.click('.panel.active .row.cursor', { button: 'right' }); await wait();
  console.log('folder menu has link:', await page.evaluate(() => $$('#menu button').some(b => b.innerText.includes('Internen Link kopieren'))));
  await closeAll();

  // A picture or text from the clipboard becomes a file.
  await page.evaluate(() => Files.go('/mnt/user/dokumente', 0)); await wait();
  await page.evaluate(() => {
    const dt = new DataTransfer();
    const c = document.createElement('canvas'); c.width = 40; c.height = 30; const x = c.getContext('2d'); x.fillStyle = '#f5c518'; x.fillRect(0, 0, 40, 30);
    const bin = atob(c.toDataURL('image/png').split(',')[1]), bytes = new Uint8Array(bin.length); for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    dt.items.add(new File([bytes], 'image.png', { type: 'image/png' }));
    document.body.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true }));
  }); await wait();
  console.log('paste picture window:', await page.evaluate(() => $('.win .notch')?.textContent), await page.inputValue('#pz-name'));
  await shot('v3a-5-paste-picture');
  await page.click('.win-foot .btn.primary'); await wait();
  console.log('picture file:', (await names()).split(', ').filter(n => n.startsWith('Screenshot')).join(', '));
  await page.evaluate(() => {
    const dt = new DataTransfer(); dt.setData('text/plain', 'Einkaufsliste\nBrot\nMilch');
    document.body.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true }));
  }); await wait();
  console.log('paste text window:', await page.evaluate(() => $('.win .notch')?.textContent), await page.inputValue('#pz-name'));
  await page.fill('#pz-name', 'Einkauf.txt'); await page.click('.win-foot .btn.primary'); await wait();
  console.log('text file:', await page.evaluate(() => FS.kids(FS.nodeAt('/mnt/user/dokumente')).find(k => k.name === 'Einkauf.txt')?.text));
  await page.evaluate(() => { Files.clip = { cut: false, from: '/mnt/user/dokumente', items: ['Einkauf.txt'] }; Files.go('/mnt/user/downloads', 0); document.body.dispatchEvent(new ClipboardEvent('paste', { clipboardData: new DataTransfer(), bubbles: true })); }); await wait(1200);
  console.log('file clip still pastes:', await page.evaluate(() => Jobs.running().length + Jobs.list.length > 0 || FS.kids(FS.nodeAt('/mnt/user/downloads')).some(k => k.name === 'Einkauf.txt')));
  await page.evaluate(() => { Files.clip = null; document.documentElement.dataset.theme = 'light'; Files.go('/mnt/user/downloads', 0); $('.panel.active .list').focus(); }); await wait();
  await page.keyboard.type('deb'); await wait();
  await shot('v3a-6-jump-light');
  console.log('shortcuts listed:', await page.evaluate(() => ['undo', 'duplicate', 'folderWithSel', 'typeJump'].map(id => CC.actions[id]?.key).join(' | ')));
};
