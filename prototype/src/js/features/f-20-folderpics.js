// A folder with a poster.jpg, folder.jpg or cover.jpg shows that picture in the card views, as media libraries name them.
(() => {
  const NAMES = ['poster.jpg', 'folder.jpg', 'cover.jpg'];
  const add = (path, name, size) => { const n = FS.nodeAt(path); if (n && !FS.kids(n).some(k => k.name === name)) n.children.push({ name, size, time: NOW - 30 * DAY, owner: 'nobody', group: 'users', mode: '0664' }); };
  add('/mnt/user/serien/Pioneer One', 'folder.jpg', 290 * KB);
  add('/mnt/user/musik/Kevin MacLeod', 'cover.jpg', 180 * KB);

  // A film or series folder gets a tall poster with its title, an album a square cover.
  function cover(dir, path) {
    const pic = FS.kids(dir).find(k => NAMES.includes(k.name.toLowerCase()));
    if (!pic) return '';
    const square = pic.name === 'cover.jpg' || path.startsWith('/mnt/user/musik');
    return square ? CC.ck.scene(dir.name, { w: 1000, h: 1000, title: dir.name }) : CC.ck.poster(dir.name, 1000, 1500);
  }
  const plain = thumb;
  thumb = (k, folder) => {
    if (!k.dir || k.link) return plain(k, folder);
    const svg = cover(k, FS.join(folder, k.name));
    return svg ? `<span class="thumb pic folderpic">${svg.replace('<svg ', '<svg class="scene" preserveAspectRatio="xMidYMid slice" ')}<span class="fbadge">${g('IconFolder')}</span></span>` : plain(k, folder);
  };

  CC.css(`
.thumb.folderpic .fbadge { position: absolute; left: .4rem; bottom: .4rem; width: 1.6rem; height: 1.6rem; border-radius: var(--radius-control); display: grid; place-items: center; background: color-mix(in srgb, var(--carbon-surface) 82%, transparent); box-shadow: var(--hairline); }
.thumb.folderpic .fbadge svg { width: .9rem; height: .9rem; color: var(--accent-glyph); }
`);
})();
