// Round 3: every window has a way out, the column menu's checkboxes, tabs as heading badges, the format glyph page.
module.exports = async (page, shot) => {
  const wait = (ms = 400) => page.waitForTimeout(ms);
  const dots = () => page.evaluate(() => [...document.querySelectorAll('.pwdot:not(.static)')].filter(d => d.getBoundingClientRect().width > 0).length);
  await wait(800);
  console.log('dots on files:', await dots());
  await shot('r3-main');

  // Open each action that may show a window and check its foot for Abbrechen or Schließen.
  const ids = await page.evaluate(() => Object.keys(CC.actions));
  const skip = new Set(['logout', 'single', 'dual', 'sidebar', 'quickview', 'fkeys', 'hidden', 'refresh', 'back', 'forward', 'up', 'home', 'swapPanels', 'equalPanels', 'newTab', 'closeTab', 'selectAll', 'invertSel', 'unselect', 'quickFilter', 'clipCut', 'clipCopy', 'clipPaste', 'copyPath', 'gotoPath', 'addBookmark', 'settings', 'openTrash', 'openShares', 'openActivity', 'about', 'mounts', 'glyphs', 'sortName', 'sortSize', 'sortTime', 'sortType', 'compareDirs', 'dupTab', 'lockTab', 'closeOtherTabs', 'palette', 'menuBar', 'nextView', 'quickLook', 'view-details', 'view-compact', 'view-list', 'view-content', 'view-tiles', 'view-large', 'view-gallery']);
  const missing = [];
  for (const id of ids) {
    if (skip.has(id)) continue;
    await page.evaluate(() => { while (winStack.length) CC.closeWin(); CC.go('files'); Files.go('/mnt/user/downloads', 0); Files.go('/mnt/user/backups', 1); Files.active = 0; const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'setup-notes.md'); CC.render(); });
    await page.evaluate((a) => { try { CC.run(a); } catch (e) { console.error(a, e.message); } }, id);
    await wait(250);
    const res = await page.evaluate(() => {
      const w = document.querySelector('.scrim .win');
      if (!w) return null;
      const labels = [...w.querySelectorAll('.win-foot .btn')].map(b => b.textContent.trim());
      return { title: w.querySelector('.notch')?.textContent, ok: labels.some(l => /^(Abbrechen|Schließen|Fertig)$/.test(l)), labels };
    });
    if (res && !res.ok) missing.push(`${id} (${res.title}): ${res.labels.join(' | ')}`);
  }
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });
  console.log('windows without a way out:', missing.length ? '\n  ' + missing.join('\n  ') : 'none');

  await page.click('.panel.active .lhead', { button: 'right' }); await wait(250);
  await page.click('#menu [role="menuitemcheckbox"]:nth-of-type(2)').catch(() => {});
  await wait(250);
  console.log('menu still open after tick:', await page.locator('#menu').count());
  await shot('r3-colmenu');
};
