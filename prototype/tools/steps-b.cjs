// Walks through every Krusader tool: search, compare, sync, diff, duplicates, usage, rename, pack, sums, split, links, rights.
module.exports = async (page, shot) => {
  const wait = (ms) => page.waitForTimeout(ms);
  const at = async (panel, path, name) => {
    await page.evaluate(([panel, path, name]) => {
      Files.active = panel;
      if (Files.T(panel).path !== path) Files.go(path, panel);
      if (name) Files.T(panel).cursor = Files.entries(panel).findIndex(e => e.name === name);
      Files.T(panel).marked = new Set();
      CC.render();
    }, [panel, path, name]);
  };
  const mark = (names) => page.evaluate((names) => { Files.T().marked = new Set(names); CC.render(); }, names);
  const run = (id) => page.evaluate((id) => CC.run(id), id);
  const esc = async () => { await page.keyboard.press('Escape'); await wait(150); };
  const primary = () => page.click('.scrim:last-of-type .win-foot .btn.primary');

  const tour = async (tag) => {
    await at(0, '/mnt/user/downloads');
    await page.keyboard.press('Alt+F7'); await wait(200);
    await page.keyboard.type('*.iso'); await wait(300); await shot(`b-${tag}search`);
    await page.keyboard.press('ArrowDown'); await page.keyboard.press('Enter'); await wait(300); await shot(`b-${tag}search-open`);

    await at(0, '/mnt/user/dokumente'); await at(1, '/mnt/user/backups/dokumente');
    await page.keyboard.press('Control+Shift+C'); await wait(250); await shot(`b-${tag}compare`);
    await primary(); await wait(300); await shot(`b-${tag}sync`);
    await page.click('#y-dir [data-v="both"]'); await wait(150); await shot(`b-${tag}sync-both`);
    await esc();
    await page.evaluate(() => { Files.marks = [{}, {}]; CC.render(); });

    await at(1, '/mnt/user/backups/dokumente', 'docker-compose.yml'); await at(0, '/mnt/user/dokumente', 'docker-compose.yml');
    await run('diff'); await wait(250); await shot(`b-${tag}diff`);
    await primary(); await primary(); await wait(250); await shot(`b-${tag}diff-next`);
    await esc();

    await run('duplicates'); await wait(1600); await shot(`b-${tag}dups`);
    if (await page.isEnabled('[data-bk-keep="new"]')) { await page.click('[data-bk-keep="new"]'); await wait(150); await shot(`b-${tag}dups-keep`); }
    await esc();

    await at(0, '/mnt/user');
    await page.keyboard.press('Alt+D'); await wait(400); await shot(`b-${tag}usage`);
    await page.click('.bk-blk.dir'); await wait(300); await shot(`b-${tag}usage-drill`);
    await esc();

    await at(0, '/mnt/user/fotos/Urlaub');
    await page.keyboard.press('Control+a'); await page.keyboard.press('Control+m'); await wait(200);
    await page.fill('#m-name', 'Urlaub_[C3]'); await page.dispatchEvent('#m-name', 'input'); await wait(150);
    await page.selectOption('#m-case', 'lower'); await wait(150); await shot(`b-${tag}rename`);
    await esc();

    await at(0, '/mnt/user/fotos/2026');
    await page.keyboard.press('+'); await wait(200); await shot(`b-${tag}pattern`);
    await esc();

    await at(0, '/mnt/user/dokumente'); await at(1, '/mnt/user/backups'); await at(0, '/mnt/user/dokumente');
    await mark(['Mietvertrag.pdf', 'Steuer 2025.xlsx', 'notizen.txt']);
    await page.keyboard.press('Alt+p'); await wait(200); await shot(`b-${tag}pack`);
    await page.click('#k-fmt [data-v="tar.zst"]'); await wait(100); await shot(`b-${tag}pack-tar`);
    await esc();

    await at(0, '/mnt/user/downloads', 'Fotos-Urlaub-2026.zip');
    await page.keyboard.press('Alt+u'); await wait(200); await shot(`b-${tag}unpack`);
    await esc();

    await at(0, '/mnt/user/isos'); await mark(['Win11_24H2_German_x64.iso', 'debian-13.1.0-amd64-netinst.iso', 'ubuntu-24.04.3-desktop-amd64.iso']);
    await run('checksum'); await wait(500); await shot(`b-${tag}sums-running`); await wait(3500); await shot(`b-${tag}sums`);
    await esc();

    await at(0, '/mnt/user/isos', 'checksums.sha256');
    await run('checksumVerify'); await wait(4200); await shot(`b-${tag}verify`);
    await esc();

    await at(0, '/mnt/user/downloads', 'Big.Buck.Bunny.2008.1080p.mkv');
    await run('split'); await wait(200); await shot(`b-${tag}split`);
    await esc();

    await at(0, '/mnt/user/downloads/unfertig', 'Fotos-2019.7z.002');
    await run('combine'); await wait(200); await shot(`b-${tag}combine`);
    await esc();

    await at(1, '/mnt/user/downloads'); await at(0, '/mnt/user/dokumente', 'Einkaufsliste.md');
    await run('symlink'); await wait(200); await shot(`b-${tag}symlink`);
    await esc();

    await at(0, '/mnt/user/fotos', '2026');
    await page.keyboard.press('Alt+Enter'); await wait(200); await shot(`b-${tag}props`);
    await esc();
    await at(0, '/mnt/user/dokumente'); await mark(['Mietvertrag.pdf', 'server.go', 'docker-compose.yml']);
    await page.keyboard.press('Alt+Enter'); await wait(200); await shot(`b-${tag}props-many`);
    await esc();
  };

  await tour('');

  // The changes the tools make have to show up in the panels.
  await at(0, '/mnt/user/dokumente'); await at(1, '/mnt/user/backups/dokumente'); await at(0, '/mnt/user/dokumente');
  await run('sync'); await wait(200); await primary(); await wait(4500);
  await page.keyboard.press('Control+Shift+C'); await wait(250); await shot('b-after-sync');
  await esc(); await esc();

  await at(0, '/mnt/user/fotos/Urlaub');
  await page.keyboard.press('Control+a'); await page.keyboard.press('Control+m'); await wait(200);
  await page.fill('#m-name', 'Urlaub_[C3]'); await page.dispatchEvent('#m-name', 'input'); await primary(); await wait(300); await shot('b-after-rename');

  await at(0, '/mnt/user/dokumente'); await at(1, '/mnt/user/backups'); await at(0, '/mnt/user/dokumente');
  await mark(['Mietvertrag.pdf', 'notizen.txt']);
  await page.keyboard.press('Alt+p'); await wait(200); await primary(); await wait(4000); await shot('b-after-pack');

  await at(0, '/mnt/user/downloads/unfertig', 'Fotos-2019.7z.001');
  await run('combine'); await wait(200); await primary(); await wait(4500); await shot('b-after-combine');

  await at(1, '/mnt/user/downloads'); await at(0, '/mnt/user/dokumente', 'Einkaufsliste.md');
  await run('symlink'); await wait(200); await page.fill('#l-name', 'einkauf.md'); await page.dispatchEvent('#l-name', 'input'); await primary(); await wait(300); await shot('b-after-symlink');

  await run('duplicates'); await wait(1600); await page.click('[data-bk-keep="new"]'); await primary(); await wait(300); await shot('b-after-dups');

  await page.evaluate(() => CC.setPref('theme', 'light')); await page.reload(); await wait(800);
  await tour('light-');
  await page.evaluate(() => CC.setPref('theme', 'dark'));

  await page.setViewportSize({ width: 420, height: 860 });
  await at(0, '/mnt/user/downloads');
  await run('search'); await wait(200); await page.keyboard.type('ubuntu'); await wait(300); await shot('b-phone-search');
  await esc();
  await run('props'); await wait(200); await shot('b-phone-props');
  await esc();
};
