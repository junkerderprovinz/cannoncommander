// Rename in the editor, own shortcuts, cleanup, metadata search and archive tests.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  const closeAll = () => page.evaluate(() => { closeMenu(); while (winStack.length) CC.closeWin(); });
  await page.setViewportSize({ width: 1908, height: 1000 });
  await wait(700);

  // Metadata search in the path row and in Alt+F7.
  const filter = async (q) => {
    await page.evaluate((q) => { Files.go('/mnt/user', 0); Object.assign(Files.T(), { qf: true, deep: true, filter: q, cursor: 0 }); CC.render(); }, q);
    await wait();
    return page.evaluate(() => Files.entries().map(k => k.name).join(', '));
  };
  console.log('9832 codec:hevc ->', await filter('codec:hevc'));
  console.log('9832 breite:>=3840 ->', await filter('breite:>=3840'));
  console.log('9832 größe:>10GB ->', await filter('größe:>10GB'));
  console.log('9832 geändert:<3d ->', await filter('geändert:<3d'));
  console.log('9832 kamera:pixel ->', await filter('kamera:pixel'));
  console.log('9832 interpret:"kevin" *.flac ->', await filter('interpret:"kevin" *.flac'));
  console.log('9832 farbe:rot ->', await filter('farbe:rot'), '| warning:', await page.evaluate(() => $('.meta-warn')?.innerText.replace(/\s+/g, ' ')));
  await shot('v3b-1-meta-warn');
  await page.evaluate(() => { Object.assign(Files.T(), { qf: false, filter: '' }); CC.render(); });
  await page.evaluate(() => CC.run('search')); await wait();
  await page.fill('#s-in', '/mnt/user');
  await page.fill('#s-name', 'jahr:>2000 *.mp3'); await wait(600);
  console.log('9832 search window:', await page.evaluate(() => $('#s-count').textContent), '|', await page.evaluate(() => $$('#s-list .bk-name').map(n => n.textContent).slice(0, 4).join(', ')));
  await page.fill('#s-name', 'farbtiefe:>8 foo:1'); await wait(600);
  console.log('9832 search window unknown:', await page.evaluate(() => $('#s-list').innerText.replace(/\s+/g, ' ')));
  await closeAll();
  await page.click('.panel.active [data-run="quickFilter"]'); await wait();
  console.log('9832 examples:', await page.evaluate(() => $$('.qdrop [data-example]').map(b => b.dataset.example).join(' | ')));
  await shot('v3b-2-examples');
  await page.click('[data-qfclose]'); await wait();

  // Rename in the editor.
  await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); CC.run('selectAll'); }); await wait();
  await page.evaluate(() => CC.run('renameEditor')); await wait();
  console.log('9820 lines:', await page.evaluate(() => $('.re-ta').value.split('\n').length), '| sum:', await page.textContent('#re-sum'));
  await page.fill('#re-q', 'IMG_'); await page.fill('#re-r', 'Urlaub_'); await wait();
  console.log('9820 hits:', await page.textContent('#re-n'));
  await page.click('#re-all'); await wait();
  console.log('9820 after replace:', await page.textContent('#re-sum'), '| enabled:', await page.evaluate(() => !$('.re-win .win-foot .btn.primary').disabled));
  await shot('v3b-3-rename-editor');
  await page.evaluate(() => { const t = $('.re-ta'); t.value += '\nzu-viel.jpg'; t.dispatchEvent(new Event('input')); }); await wait();
  console.log('9820 extra line:', await page.evaluate(() => $('#re-warn').hidden ? 'no warning' : $('#re-warn').textContent), '| disabled:', await page.evaluate(() => $('.re-win .win-foot .btn.primary').disabled));
  await page.evaluate(() => { const t = $('.re-ta'); t.value = t.value.replace(/\nzu-viel\.jpg$/, ''); const lines = t.value.split('\n'); lines[1] = lines[0]; t.value = lines.join('\n'); t.dispatchEvent(new Event('input')); }); await wait();
  console.log('9820 duplicate:', await page.evaluate(() => $$('#re-list .pill.fault').map(p => p.textContent).join(', ')));
  await page.evaluate(() => { const t = $('.re-ta'); t.value = t.value.split('\n').map((l, i) => i === 1 ? 'Urlaub_zwei.jpg' : l).join('\n'); t.dispatchEvent(new Event('input')); }); await wait();
  await page.click('.re-win .win-foot .btn.primary'); await wait();
  console.log('9820 renamed:', await page.evaluate(() => Files.entries().map(k => k.name).slice(0, 4).join(', ')));
  await page.evaluate(() => { CC.run('selectAll'); CC.run('multiRename'); }); await wait();
  await page.click('.win-foot .btn:has-text("Im Editor")'); await wait(500);
  console.log('9820 from multi rename:', await page.evaluate(() => $('.win .notch')?.textContent));
  await closeAll();

  // Own shortcuts.
  await page.keyboard.press('F1'); await wait();
  console.log('9821 window:', await page.evaluate(() => `${$('.win .notch').textContent}, ${$$('.kb-row').length} rows`));
  await page.fill('#kb-q', 'kopieren'); await wait();
  console.log('9821 filter:', await page.evaluate(() => $$('.kb-row .kb-name').map(n => n.textContent.trim()).join(' | ')));
  await page.fill('#kb-q', ''); await wait();
  await page.click('[data-kb-rec="copy"]'); await wait();
  await page.keyboard.press('Control+k'); await wait();
  console.log('9821 conflict:', await page.evaluate(() => $('.kb-warn')?.textContent));
  await shot('v3b-4-keys-conflict');
  await page.keyboard.press('Control+k'); await wait();
  console.log('9821 copy now:', await page.evaluate(() => $('[data-kb-rec="copy"]').textContent.trim()), '| palette:', await page.evaluate(() => $('[data-kb-rec="palette"]').textContent.trim()));
  await page.click('[data-kb-rec="newFolder"]'); await page.keyboard.press('x'); await wait();
  console.log('9821 single letter:', await page.evaluate(() => $('.kb-warn')?.textContent));
  await page.keyboard.press('Control+Alt+n'); await wait();
  await page.click('.kb-win .win-foot .btn.primary'); await wait();
  console.log('9821 applied:', await page.evaluate(() => `copy ${CC.actions.copy.key}, newFolder ${CC.actions.newFolder.key}, palette ${CC.actions.palette.key || 'none'}, Ctrl+k ${CC.keymap['Ctrl+k']}, F5 ${CC.keymap.F5 || 'none'}`));
  console.log('9821 fkeys:', await page.evaluate(() => $$('#fkeys .btn').map(b => b.innerText.replace(/\s+/g, ' ').trim()).join(' | ')));
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); Files.T().cursor = 1; CC.render(); }); await wait();
  await page.click('.panel.active .row.cursor', { button: 'right' }); await wait();
  console.log('9821 menu key:', await page.evaluate(() => $$('#menu button').find(b => b.textContent.startsWith('Kopieren'))?.querySelector('.kbd')?.textContent));
  await page.keyboard.press('Escape'); await wait();
  await page.keyboard.press('Control+k'); await wait();
  console.log('9821 Ctrl+k runs:', await page.evaluate(() => $('.win .notch')?.textContent || ($('.pal-scrim') ? 'palette' : 'nothing')));
  await closeAll();
  console.log('9821 stored:', await page.evaluate(() => !!localStorage.getItem('cannoncommander-keys')));
  await page.reload(); await wait(900);
  console.log('9821 after reload:', await page.evaluate(() => `copy ${CC.actions.copy.key}, Ctrl+k ${CC.keymap['Ctrl+k']}`));
  await page.keyboard.press('F1'); await wait();
  await page.click('.kb-win .win-foot .btn:has-text("Alles zurücksetzen")'); await wait();
  await page.click('.kb-win .win-foot .btn.primary'); await wait();
  console.log('9821 reset:', await page.evaluate(() => `copy ${CC.actions.copy.key}, palette ${CC.actions.palette.key}, stored ${!!localStorage.getItem('cannoncommander-keys')}`));
  await closeAll();

  // Cleanup.
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); CC.run('cleanup'); }); await wait(600);
  console.log('9831 groups:', await page.evaluate(() => $$('.cl-head').map(h => h.innerText.replace(/\s+/g, ' ').trim()).join(' | ')));
  await shot('v3b-6-cleanup');
  await page.click('[data-cl-group="temp"]'); await page.click('[data-cl-group="name"]'); await page.click('[data-cl-group="ext"]'); await wait();
  console.log('9831 primary:', await page.evaluate(() => $('.bk-clean .win-foot .btn.primary').textContent.trim()));
  await page.click('.bk-clean .win-foot .btn.primary'); await wait(500);
  console.log('9831 delete window:', await page.evaluate(() => $('.xwin .xdel b')?.textContent));
  await page.click('.xwin .win-foot .btn.primary'); await wait(500);
  console.log('9831 left:', await page.evaluate(() => $$('.cl-head').map(h => h.innerText.replace(/\s+/g, ' ').trim()).join(' | ')));
  console.log('9831 unfertig now:', await page.evaluate(() => FS.kids(FS.nodeAt('/mnt/user/downloads/unfertig')).map(k => JSON.stringify(k.name)).join(', ')));
  await closeAll();

  // Archive test.
  await page.evaluate(() => { Files.go('/mnt/user/backups', 0); const t = Files.T(); t.marked = new Set(Files.entries().filter(k => /\.(zst|zip)$/.test(k.name)).map(k => k.name)); CC.render(); }); await wait();
  await page.evaluate(() => CC.run('testArchive')); await wait(400);
  await shot('v3b-7-archive-running');
  await wait(3500);
  console.log('9833 result:', await page.evaluate(() => `${$('#at-sum').textContent} | ${$$('.at-row').map(r => r.innerText.replace(/\s+/g, ' ').trim()).join(' | ')}`));
  await shot('v3b-8-archive-done');
  await closeAll(); await wait();
  console.log('9833 list mark:', await page.evaluate(() => $$('.row.damaged').map(r => r.innerText.replace(/\s+/g, ' ').trim()).join(' | ')));
  await page.evaluate(() => { const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name.endsWith('.zip')); t.marked = new Set(); CC.render(); }); await wait();
  await page.click('.panel.active .row.cursor', { button: 'right' }); await wait();
  console.log('9833 context menu:', await page.evaluate(() => $$('#menu button').map(b => b.textContent.trim()).filter(t => /Archiv|Entpacken/.test(t)).join(' | ')));
  await page.keyboard.press('Escape');
  await shot('v3b-9-damaged');
  await page.evaluate(() => { document.documentElement.dataset.theme = 'light'; CC.run('cleanup'); }); await wait(600);
  await shot('v3b-10-cleanup-light');
};
