// Steps for the viewer, editor, transfers, trash, conflicts and sign-in.
module.exports = async (page, shot) => {
  const wait = (ms) => page.waitForTimeout(ms);
  const at = (path, name, panel = 0) => page.evaluate(([p, n, i]) => {
    Files.active = i; Files.go(p, i);
    Files.T(i).cursor = Files.entries(i).findIndex(e => e.name === n);
    CC.render();
  }, [path, name, panel]);
  const closeAll = () => page.evaluate(() => { while (winStack.length) winStack.at(-1).close(); document.getElementById('menu')?.remove(); });
  const view = async (path, name, file, extra) => { await at(path, name); await page.keyboard.press('F3'); await wait(250); if (extra) await extra(); await shot(file); await closeAll(); };
  const primary = () => page.locator('.scrim').last().locator('.win-foot .btn.primary').click();
  const theme = (t) => page.evaluate((x) => CC.setPref('theme', x), t);

  const tour = async (pre) => {
    await view('/mnt/user/fotos/2026', 'IMG_0203.jpg', pre + 'view-image', async () => { await page.keyboard.press('ArrowRight'); await wait(150); });
    await view('/mnt/user/filme/Big Buck Bunny (2008)', 'Big Buck Bunny (2008).mkv', pre + 'view-video', async () => {
      await page.click('.cv-screen'); await page.click('[data-pick="sub"]'); await wait(100);
      await page.click('#menu button:has-text("Deutsch")'); await wait(3800);
    });
    await view('/mnt/user/dokumente', 'Mietvertrag.pdf', pre + 'view-pdf', async () => { await page.keyboard.press('PageDown'); await wait(600); });
    await view('/mnt/user/downloads', 'setup-notes.md', pre + 'view-md');
  };

  await tour('c-');
  await view('/mnt/user/fotos/2026', 'IMG_0203.jpg', 'c-view-image-100', async () => { await page.click('#cv-zoom button[data-v="full"]'); await wait(200); });
  await view('/mnt/user/musik/Kevin MacLeod', 'Sneaky Snitch.mp3', 'c-view-audio', async () => { await page.click('.cv-playbtn'); await wait(2500); });
  await view('/mnt/user/dokumente', 'server.go', 'c-view-code');
  await view('/mnt/user/dokumente', 'Steuer 2025.xlsx', 'c-view-xlsx', async () => { await page.click('[data-rc="13,1"]'); });
  await view('/mnt/user/dokumente', 'Präsentation.pptx', 'c-view-pptx', async () => { await page.keyboard.press('ArrowRight'); await page.keyboard.press('ArrowRight'); });
  await view('/mnt/user/dokumente', 'Bewerbung.docx', 'c-view-docx');
  await view('/mnt/user/downloads', 'Rechnung-Oktober.pdf', 'c-view-invoice');
  await view('/mnt/user/downloads', 'Fotos-Urlaub-2026.zip', 'c-view-zip');
  await view('/mnt/user/backups', 'appdata-2026-10-31.tar.zst', 'c-view-tar');
  await view('/mnt/user/isos', 'Win11_24H2_German_x64.iso', 'c-view-iso');
  await view('/mnt/user/backups', 'vm-win11-2026-10-01.img', 'c-view-img');
  await view('/mnt/user/appdata/bombvault', 'bombvault.db', 'c-view-hex');
  await view('/mnt/user/appdata/bombvault/logs', 'engine.log', 'c-view-log');

  // Preview bar beside the list.
  await page.evaluate(() => { CC.prefs.dual = true; CC.setPref('quickview', true); });
  await at('/mnt/user/fotos/2026', 'IMG_0207.jpg'); await shot('c-preview-image');
  await at('/mnt/user/downloads', 'setup-notes.md'); await shot('c-preview-md');
  await at('/mnt/user/fotos', '2026'); await shot('c-preview-dir');
  await at('/mnt/user/filme/Sintel (2010)', 'Sintel (2010).mkv'); await shot('c-preview-video');
  await at('/mnt/user/musik/Kevin MacLeod', 'Cipher.mp3'); await shot('c-preview-audio');
  await page.evaluate(() => CC.setPref('quickview', false));

  // Editor: find, save, unsaved warning.
  await at('/mnt/user/dokumente', 'notizen.txt');
  await page.keyboard.press('F4'); await wait(150);
  await page.keyboard.press('End'); await page.keyboard.type(' (erledigt)');
  await page.keyboard.press('Control+f'); await page.keyboard.type('disk'); await wait(100);
  await shot('c-edit-find');
  await page.click('#ce-r'); await page.keyboard.type('Platte'); await page.click('#ce-all'); await wait(100);
  await page.keyboard.press('Escape'); await page.keyboard.press('Control+s'); await wait(100);
  await page.keyboard.type('x'); await page.keyboard.press('Escape'); await wait(150);
  await shot('c-edit-unsaved');
  await page.locator('.scrim').last().locator('button:has-text("Verwerfen")').click(); await wait(150);
  const saved = await page.evaluate(() => FS.nodeAt('/mnt/user/dokumente/notizen.txt').text.slice(0, 80));
  console.log('saved text starts:', JSON.stringify(saved));
  await at('/mnt/user/downloads', 'setup-notes.md');
  await page.keyboard.press('F4'); await wait(300); await shot('c-edit-md'); await closeAll();
  await at('/mnt/user/dokumente', 'notizen.txt');
  await page.keyboard.press('Shift+F4'); await wait(150); await page.keyboard.type('todo'); await page.keyboard.press('Enter'); await wait(200);
  await page.keyboard.type('# Todo\n\n- Kabel kaufen\n'); await page.keyboard.press('Control+s'); await wait(100);
  await shot('c-edit-new'); await closeAll();

  // Upload with a clash, then the queue.
  await at('/mnt/user/downloads', 'setup-notes.md');
  await page.evaluate(() => CC.run('upload')); await wait(150); await shot('c-upload-empty');
  await page.setInputFiles('#cu-in', [
    { name: 'setup-notes.md', mimeType: 'text/markdown', buffer: Buffer.from('# Neu\n\nHochgeladen.\n') },
    { name: 'urlaub-video.mp4', mimeType: 'video/mp4', buffer: Buffer.alloc(900 * 1024) },
    { name: 'liste.txt', mimeType: 'text/plain', buffer: Buffer.from('eins\nzwei\n') },
  ]);
  await wait(200); await shot('c-conflict-upload');
  await page.click('#cf-all button[data-v="both"]'); await primary(); await wait(300);
  await shot('c-upload-progress');
  await wait(2500); await shot('c-upload-done'); await closeAll();
  console.log('downloads now:', await page.evaluate(() => FS.kids(FS.nodeAt('/mnt/user/downloads')).map(k => k.name).filter(n => /setup|liste|urlaub/.test(n)).join(', ')));

  // Download and share.
  await at('/mnt/user/dokumente', 'Mietvertrag.pdf');
  await page.evaluate(() => { const t = Files.T(); t.marked = new Set(['Mietvertrag.pdf', 'Bewerbung.docx', 'notizen.txt']); CC.render(); CC.run('download'); });
  await wait(150); await shot('c-download'); await closeAll();
  await at('/mnt/user/fotos', 'Urlaub');
  await page.evaluate(() => CC.run('share')); await wait(150);
  await page.click('#cs-pw'); await page.keyboard.type('sommer2026'); await page.click('[data-eye]');
  await shot('c-share');
  await primary(); await wait(200); await shot('c-share-done'); await closeAll();

  // Copy onto a name that exists.
  await at('/mnt/user/downloads', 'ubuntu-24.04.3-desktop-amd64.iso');
  await page.evaluate(() => { Files.go('/mnt/user/backups', 1); Files.active = 0; const t = Files.T(); t.marked = new Set(['ubuntu-24.04.3-desktop-amd64.iso', 'album.flac']); CC.render(); });
  await page.keyboard.press('F5'); await wait(150); await primary(); await wait(200);
  await shot('c-conflict-copy'); await closeAll();

  // Trash.
  await at('/mnt/user/dokumente', 'Einkaufsliste.md');
  await page.keyboard.press('F8'); await wait(100); await primary(); await wait(100);
  await page.evaluate(() => CC.run('openTrash')); await wait(150); await shot('c-trash');
  await page.click('.ct-row[data-i="0"]'); await page.click('.ct-row[data-i="1"]', { modifiers: ['Control'] }); await wait(100);
  await shot('c-trash-picked');
  await page.click('#ct-restore'); await wait(150);
  console.log('restored back:', await page.evaluate(() => !!FS.nodeAt('/mnt/user/dokumente/Einkaufsliste.md')), 'trash left', await page.evaluate(() => DATA.trash.length));
  await page.click('#ct-more'); await wait(100); await shot('c-trash-more');
  await page.click('#menu button:has-text("leeren")'); await wait(100); await shot('c-trash-empty-ask');
  await primary(); await wait(150); await shot('c-trash-empty');

  // Sign-in.
  await page.evaluate(() => CC.run('logout')); await wait(200); await shot('c-login');
  await page.fill('#cl-pass', 'geheim'); await page.keyboard.press('Enter'); await wait(150); await shot('c-login-totp');
  await page.keyboard.type('123456'); await wait(200);
  console.log('signed in:', await page.evaluate(() => CC.signedIn));
  await page.evaluate(() => CC.run('logout')); await wait(100);
  await page.click('[data-cl="forgot"]'); await wait(100); await page.fill('#cl-code', 'abcd'); await page.keyboard.press('Enter'); await wait(100);
  await shot('c-login-reset');
  await page.evaluate(() => { CC.signedIn = true; CC.go('files'); });

  // Light theme.
  await theme('light');
  await tour('c-light-');
  await at('/mnt/user/dokumente', 'notizen.txt'); await page.keyboard.press('F4'); await wait(200); await page.keyboard.press('Control+f'); await page.keyboard.type('a'); await shot('c-light-edit'); await closeAll();
  await page.evaluate(() => CC.run('upload')); await page.evaluate(() => CC.uploadFiles([new File(['x'.repeat(40e6)], 'gross.bin'), new File(['y'], 'klein.txt')], '/mnt/user/downloads')); await wait(500); await shot('c-light-upload'); await wait(3000); await closeAll();
  await page.evaluate(() => { DATA.trash.push({ name: 'alt.txt', from: '/mnt/user/dokumente', size: 3000, deleted: NOW - DAY, by: 'anna' }); CC.run('openTrash'); }); await wait(100); await shot('c-light-trash');
  await page.evaluate(() => CC.run('logout')); await wait(100); await shot('c-light-login');
  await page.evaluate(() => { CC.signedIn = true; CC.go('files'); });

  // Phone width.
  await theme('dark');
  await page.setViewportSize({ width: 390, height: 844 }); await wait(200);
  await tour('c-phone-');
  await view('/mnt/user/musik/Kevin MacLeod', 'Sneaky Snitch.mp3', 'c-phone-audio');
  await at('/mnt/user/dokumente', 'notizen.txt'); await page.keyboard.press('F4'); await wait(200); await page.keyboard.press('Control+f'); await shot('c-phone-edit'); await closeAll();
  await page.evaluate(() => CC.run('openTrash')); await wait(100); await shot('c-phone-trash');
  await page.evaluate(() => CC.go('files'));
  await at('/mnt/user/fotos', 'Urlaub'); await page.evaluate(() => CC.run('share')); await wait(100); await shot('c-phone-share'); await closeAll();
  await at('/mnt/user/downloads', 'ubuntu-24.04.3-desktop-amd64.iso');
  await page.evaluate(() => { Files.T(1).path = '/mnt/user/backups'; Files.transfer('copy', [Files.cursorEntry()], '/mnt/user/backups'); }); await wait(150); await shot('c-phone-conflict'); await closeAll();
  await page.evaluate(() => CC.run('upload')); await page.evaluate(() => CC.uploadFiles([new File(['x'.repeat(30e6)], 'video.mp4')], '/mnt/user/downloads')); await wait(400); await shot('c-phone-upload'); await closeAll();
  await page.evaluate(() => CC.run('logout')); await wait(100); await shot('c-phone-login');
};
