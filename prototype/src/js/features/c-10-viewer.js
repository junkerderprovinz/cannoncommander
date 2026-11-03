// Ansehen (F3) and the preview bar: one renderer per kind of file, in a large window or compact beside the list.
(() => {
  const { real, fmtDur, plural, winKeys, viewKind, editable, picture, poster, dims, rng, SUBS } = CC.ck;
  const R = CC.ck.R = {};

  const siblings = (folder, kind) => {
    const list = folder === Files.T().path ? Files.entries() : FS.kids(FS.nodeAt(folder)).slice().sort((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }));
    return list.filter(k => !k.dir && !k.up && viewKind(k) === kind && (CC.prefs.showHidden || !k.name.startsWith('.'))).map(k => real(folder, k.name) || k);
  };

  function current() {
    const c = Files.cursorEntry();
    const k = c && !c.up && !c.dir ? c : Files.sel().find(x => !x.dir);
    return k && { node: real(Files.T().path, k.name) || k, folder: Files.T().path };
  }

  function openViewer(node, folder) {
    const kind = viewKind(node);
    const sibs = ['img', 'audio'].includes(kind) ? siblings(folder, kind) : [node];
    if (!sibs.includes(node)) sibs.unshift(node);
    let cur = node, live = {};
    const inArch = FS.inArchive(folder);
    const download = { label: 'Herunterladen', g: 'IconDownload', run: () => { CC.ck.download([cur], folder); return false; } };
    const primary =
      editable(node) && !inArch ? { label: 'Bearbeiten', g: 'IconPencil', primary: true, run: () => { setTimeout(() => CC.ck.edit(cur, folder), 0); } }
      : kind === 'office' ? { label: 'In OnlyOffice öffnen', g: 'IconExternalLink', primary: true, run: () => { CC.toast(`OnlyOffice öffnet ${cur.name} in einem neuen Tab`); return false; } }
      : kind === 'arch' && CC.actions.unpack && !inArch ? { label: 'Entpacken', g: 'IconArchive', primary: true, run: () => { setTimeout(() => CC.run('unpack'), 0); } }
      : { ...download, primary: true };
    const foot = [{ start: '<span class="cv-meta"></span>' }, closeBtn, { label: 'Freigeben', g: 'IconLink', run: () => { CC.ck.share([cur], folder); return false; } }, ...(primary.label === download.label ? [] : [download]), primary];
    const w = win({ title: node.name, full: true, cls: `cv-win cvk-${kind}`, body: '<div class="cv-host"></div>', foot, onClose: () => live.stop?.() });
    const host = $('.cv-host', w);
    const show = (n) => {
      live.stop?.();
      cur = n;
      $('.notch', w).textContent = n.name;
      host.innerHTML = '<div class="cv-box"></div>';
      live = (R[kind] || R.file)(host.firstChild, n, { folder, sibs, go: show, compact: false }) || {};
      $('.cv-meta', w).textContent = [live.info, fmtSize(n.size), `geändert ${fmtTime(n.time)}`].filter(Boolean).join(', ');
      wireControls(host);
    };
    show(node);
    // The core focuses the last foot button; the stage takes the keys for the gallery and the players instead.
    setTimeout(() => host.focus({ preventScroll: true }), 30);
    host.tabIndex = -1;
    winKeys(w, (e, combo) => live.keys?.(e, combo));
    return w;
  }

  CC.action('view', { label: 'Ansehen', g: 'IconEye', key: 'F3', needs: 'file', run: () => {
    const c = current();
    if (c) openViewer(c.node, c.folder);
  } });
  CC.ck.view = openViewer;
  CC.ck.current = current;

  // The preview bar: the same renderers, compact, followed by the file's details.
  CC.renderPreview = (entry, host, folder, { autoplay = false } = {}) => {
    const n = real(folder, entry.name) || entry;
    const kind = n.dir ? 'dir' : viewKind(n);
    const box = document.createElement('div');
    box.className = `cv-compact cvk-${kind}`;
    host.appendChild(box);
    const live = (R[kind] || R.file)(box, n, { folder: n.dir ? FS.join(folder, n.name) : folder, sibs: [n], go() {}, compact: true, autoplay }) || {};
    const rows = [
      ['Größe', fmtSize(FS.totalSize(n)) || '0 B'],
      ...(live.info ? [[kind === 'dir' ? 'Inhalt' : 'Format', live.info]] : []),
      ['Geändert', fmtTime(n.time)],
      ['Besitzer', `${n.owner || 'nobody'}:${n.group || 'users'}`],
      ['Rechte', n.mode || '0664'],
    ];
    host.insertAdjacentHTML('beforeend', `<div class="well cv-details"><div class="eyebrow">${esc(n.name)}</div>${rows.map(([l, v]) => frow(l, `<span class="pill glim-num">${esc(v)}</span>`)).join('')}</div>`);
    wireControls(host);
  };

  let zoom = 'fit';
  R.img = (host, n, ctx) => {
    const [w, h] = n.url ? [0, 0] : dims(n, ctx.folder);
    const info = w ? `${w} × ${h}` : '';
    if (ctx.compact) { host.innerHTML = `<div class="cv-pic-c">${picture(n, ctx.folder)}</div>`; return { info }; }
    const i = ctx.sibs.indexOf(n), many = ctx.sibs.length > 1;
    host.innerHTML = `<div class="cv-bar">${many ? `<span class="pill glim-num">${i + 1} von ${ctx.sibs.length}</span>` : ''}<span class="grow"></span>
        ${n.url ? '' : seg([['fit', 'Einpassen', 'IconCompact'], ['full', '100 %', 'IconSearch']], zoom, 'id="cv-zoom"')}</div>
      <div class="cv-stage ${n.url ? 'fit' : zoom}" id="cv-stage"><div class="cv-pic" style="${zoom === 'full' && w ? `width:${w}px;height:${h}px` : `--ar:${w || 4} / ${h || 3}`}">${picture(n, ctx.folder)}</div>
        ${many ? `<button class="cv-nav prev" data-step="-1" data-tip="Vorheriges Bild (Pfeil links)" aria-label="Vorheriges Bild">${g('IconBack')}</button>
        <button class="cv-nav next" data-step="1" data-tip="Nächstes Bild (Pfeil rechts)" aria-label="Nächstes Bild">${g('IconForward')}</button>` : ''}</div>
      ${many ? `<div class="cv-strip">${ctx.sibs.map((s, k) => `<button class="cv-thumb ${s === n ? 'cur' : ''}" data-k="${k}" aria-label="${esc(s.name)}">${picture(s, ctx.folder)}</button>`).join('')}</div>` : ''}`;
    const stage = $('#cv-stage', host);
    const step = (d) => ctx.go(ctx.sibs[(i + d + ctx.sibs.length) % ctx.sibs.length]);
    host.onclick = (e) => {
      const s = e.target.closest('[data-step]'), t = e.target.closest('[data-k]');
      if (s) step(+s.dataset.step);
      if (t) ctx.go(ctx.sibs[+t.dataset.k]);
    };
    $('#cv-zoom', host)?.addEventListener('change', (e) => { zoom = segVal(e.currentTarget); ctx.go(n); });
    if (zoom === 'full') {
      requestAnimationFrame(() => { stage.scrollLeft = (stage.scrollWidth - stage.clientWidth) / 2; stage.scrollTop = (stage.scrollHeight - stage.clientHeight) / 2; });
      let pan = null;
      stage.onmousedown = (e) => { if (e.target.closest('button')) return; pan = { x: e.clientX, y: e.clientY, l: stage.scrollLeft, t: stage.scrollTop }; stage.classList.add('panning'); };
      stage.onmousemove = (e) => { if (pan) { stage.scrollLeft = pan.l - (e.clientX - pan.x); stage.scrollTop = pan.t - (e.clientY - pan.y); } };
      stage.onmouseup = stage.onmouseleave = () => { pan = null; stage.classList.remove('panning'); };
    }
    $('.cv-thumb.cur', host)?.scrollIntoView({ block: 'nearest', inline: 'center' });
    return {
      info,
      keys: (e, combo) => {
        if (combo === 'ArrowRight' || combo === 'PageDown') return step(1), true;
        if (combo === 'ArrowLeft' || combo === 'PageUp') return step(-1), true;
        if (combo === 'Home') return ctx.go(ctx.sibs[0]), true;
        if (combo === 'End') return ctx.go(ctx.sibs.at(-1)), true;
      },
    };
  };

  // A player that only pretends: time runs on a timer and the poster drifts a little while it plays.
  function player(host, n, ctx, { dur, render, frame }) {
    const st = { t: 0, playing: false, dur };
    let timer = null;
    const tick = () => {
      if (!host.isConnected) return stop();
      st.t = Math.min(st.dur, st.t + .25);
      if (st.t >= st.dur) { st.playing = false; clearInterval(timer); timer = null; }
      paint();
    };
    const paint = () => {
      $$('[data-pi]', host).forEach(b => { b.innerHTML = g(st.playing ? 'IconPause' : 'IconPlay'); b.dataset.tip = st.playing ? 'Anhalten (Leertaste)' : 'Abspielen (Leertaste)'; b.setAttribute('aria-label', st.playing ? 'Anhalten' : 'Abspielen'); });
      const pos = $('.cv-now', host); if (pos) pos.textContent = fmtDur(st.t);
      $$('.cv-scrub', host).forEach(s => s.style.setProperty('--p', (st.t / st.dur * 100).toFixed(2) + '%'));
      host.classList.toggle('playing', st.playing);
      frame && frame(st);
    };
    const toggle = () => {
      st.playing = !st.playing;
      if (st.playing && st.t >= st.dur) st.t = 0;
      clearInterval(timer);
      timer = st.playing ? setInterval(tick, 250) : null;
      paint();
    };
    const seek = (t) => { st.t = Math.max(0, Math.min(st.dur, t)); paint(); };
    const stop = () => { clearInterval(timer); timer = null; };
    render(st);
    host.addEventListener('click', (e) => {
      if (e.target.closest('[data-play]')) toggle();
    });
    $$('.cv-scrub', host).forEach(s => {
      const at = (e) => { const r = s.getBoundingClientRect(); seek((e.clientX - r.left) / r.width * st.dur); };
      s.addEventListener('pointerdown', (e) => { at(e); s.setPointerCapture(e.pointerId); s.onpointermove = at; });
      s.addEventListener('pointerup', () => { s.onpointermove = null; });
    });
    paint();
    return {
      st, stop, toggle, seek,
      keys: (e, combo) => {
        if ((combo === ' ' && !e.target.closest('button, input, textarea')) || combo === 'k') return toggle(), true;
        if (combo === 'ArrowRight') return seek(st.t + 10), true;
        if (combo === 'ArrowLeft') return seek(st.t - 10), true;
      },
    };
  }

  const KNOWN_DUR = { 'Big Buck Bunny': 596, Sintel: 888, 'Tears of Steel': 734, 'Elephants Dream': 653 };
  const filmOf = (n, folder) => { const f = FS.split(folder).at(-1) || ''; return /\(\d{4}\)/.test(f) ? f : n.name; };
  const prefs = { sub: 'off', audio: 0 };

  R.video = (host, n, ctx) => {
    const film = filmOf(n, ctx.folder);
    const title = Object.keys(KNOWN_DUR).find(k => film.replace(/\./g, ' ').startsWith(k));
    const dur = KNOWN_DUR[title] || Math.round(n.size / MB / 2.2) + 300;
    const info = `1920 × 1080, H.264, ${fmtDur(dur)}`;
    if (ctx.compact && ctx.autoplay) {
      const p = player(host, n, ctx, { dur, render: () => {
        host.innerHTML = `<div class="cv-screen compact" data-play>${poster(film)}<span class="cv-big">${g('IconPlay')}</span></div>
          <div class="cv-vbar"><span class="glim-num cv-now">0:00</span><span class="cv-scrub" role="slider" aria-label="Position"><i></i></span><span class="glim-num muted">${fmtDur(dur)}</span></div>`;
      } });
      p.toggle();
      return { ...p, info };
    }
    if (ctx.compact) {
      host.innerHTML = `<button class="cv-screen compact" data-run="view" data-tip="Im großen Fenster abspielen (F3)">${poster(film)}<span class="cv-big">${g('IconPlay')}</span></button>`;
      return { info };
    }
    const srt = FS.kids(FS.nodeAt(ctx.folder)).some(k => FS.ext(k.name) === 'srt');
    const subs = [['off', 'Aus'], ...(srt ? [['de', 'Deutsch']] : []), ['en', 'Englisch, eingebettet']];
    const audios = ['Englisch, 5.1', 'Englisch, Stereo', 'Kommentar der Regie'];
    if (!subs.some(s => s[0] === prefs.sub)) prefs.sub = 'off';
    const subLabel = () => subs.find(s => s[0] === prefs.sub)[1];
    const p = player(host, n, ctx, {
      dur,
      render: () => {
        host.innerHTML = `<div class="cv-video">
          <div class="cv-screen" data-play>${poster(film, 1600, 900, false)}<div class="cv-vtitle"><b>${esc(CC.ck.filmTitle(film).name)}</b><span class="glim-num">${CC.ck.filmTitle(film).year}</span></div><span class="cv-big">${g('IconPlay')}</span><div class="cv-sub"></div></div>
          <div class="cv-ctrl">
            <button class="btn icon" data-play data-pi aria-label="Abspielen"></button>
            <span class="glim-num cv-now">0:00</span>
            <div class="cv-scrub" role="slider" aria-label="Position" tabindex="0"><i></i><b></b></div>
            <span class="glim-num muted">${fmtDur(dur)}</span>
            <button class="btn" data-pick="sub" data-tip="Untertitel">${g('IconFileSubtitles')}<span>${esc(subLabel())}</span></button>
            <button class="btn" data-pick="audio" data-tip="Tonspur">${g('IconFileAudio')}<span>${esc(audios[prefs.audio])}</span></button>
          </div></div>`;
      },
      frame: (st) => {
        const svg = $('.cv-screen > svg', host);
        if (svg) svg.style.transform = `scale(${1.04 + .06 * st.t / st.dur}) translateX(${(-2 + 4 * st.t / st.dur).toFixed(2)}%)`;
        const line = prefs.sub === 'off' || st.t < 3 ? '' : SUBS[Math.floor((st.t - 3) / 4) % SUBS.length];
        const box = $('.cv-sub', host);
        if (box) box.textContent = prefs.sub === 'en' && line ? EN[SUBS.indexOf(line)] : line;
      },
    });
    host.addEventListener('click', (e) => {
      const b = e.target.closest('[data-pick]');
      if (!b) return;
      const r = b.getBoundingClientRect();
      const items = b.dataset.pick === 'sub'
        ? [{ head: 'Untertitel' }, ...subs.map(([v, l]) => ({ label: l, checked: () => prefs.sub === v, run: () => { prefs.sub = v; $('span', b).textContent = l; p.seek(p.st.t); } }))]
        : [{ head: 'Tonspur' }, ...audios.map((l, k) => ({ label: l, checked: () => prefs.audio === k, run: () => { prefs.audio = k; $('span', b).textContent = l; } }))];
      // The shell closes menus on every click, so this one opens once the click is done.
      setTimeout(() => menu(r.left, r.top, items, { above: true }), 0);
    });
    return { ...p, info };
  };
  const EN = ['Do you hear that?', 'Someone is in the forest.', 'Stay close behind me.', 'We should turn back before it gets dark.', 'No. We keep going.', 'I have never seen anything like it.', 'Quick, this way!', 'Is it over?', 'For today, yes.', "Then let's go home."];

  const coverOf = (n, folder) => {
    const r = rng(folder), hue = Math.floor(r() * 360), artist = FS.split(folder).at(-1) || '';
    return `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="cv${CC.ck.hash(folder + n.name)}" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(${hue} 55% 55%)"/><stop offset="1" stop-color="hsl(${(hue + 60) % 360} 50% 22%)"/></linearGradient></defs>
      <rect width="100" height="100" fill="url(#cv${CC.ck.hash(folder + n.name)})"/>${Array.from({ length: 5 }, (_, k) => `<circle cx="${30 + r() * 40}" cy="${30 + r() * 40}" r="${8 + k * 9}" fill="none" stroke="#fff" stroke-opacity="${.08 + r() * .12}" stroke-width="${1 + r() * 3}"/>`).join('')}
      <text x="8" y="92" fill="#fff" fill-opacity=".85" font-family="Noto Sans, sans-serif" font-size="7" font-weight="700" letter-spacing=".6">${esc(artist.toUpperCase().slice(0, 22))}</text></svg>`;
  };

  R.audio = (host, n, ctx) => {
    const dur = Math.round(n.size / (FS.ext(n.name) === 'flac' ? 110 * KB : 40 * KB));
    const name = n.name.replace(/\.[^.]+$/, ''), artist = FS.split(ctx.folder).at(-1);
    const info = `${FS.ext(n.name).toUpperCase()}, ${fmtDur(dur)}`;
    const r = rng(n.name), bars = Array.from({ length: ctx.compact ? 160 : 260 }, (_, k) => .14 + Math.abs(Math.sin(k * .11 + r() * .6)) * .5 + Math.abs(Math.sin(k * .53)) * .16 + r() * .2);
    const i = ctx.sibs.indexOf(n), many = ctx.sibs.length > 1;
    const p = player(host, n, ctx, {
      dur,
      render: () => {
        host.innerHTML = `<div class="cv-audio ${ctx.compact ? 'compact' : ''}">
          <div class="cv-cover">${coverOf(n, ctx.folder)}</div>
          <div class="cv-atitle"><div class="t">${esc(name)}</div><div class="muted">${esc(artist)}</div></div>
          <div class="cv-scrub cv-wave" role="slider" aria-label="Position" tabindex="0">${bars.map(h => `<i style="height:${(h * 100).toFixed(0)}%"></i>`).join('')}</div>
          <div class="cv-actl">
            <span class="glim-num cv-now">0:00</span>
            <span class="cv-tr">
              ${many ? `<button class="btn icon" data-step="-1" data-tip="Vorheriger Titel" aria-label="Vorheriger Titel">${g('IconFirst')}</button>` : ''}
              <button class="btn icon cv-playbtn" data-play data-pi aria-label="Abspielen"></button>
              ${many ? `<button class="btn icon" data-step="1" data-tip="Nächster Titel" aria-label="Nächster Titel">${g('IconLatest')}</button>` : ''}
            </span>
            <span class="glim-num muted">${fmtDur(dur)}</span>
          </div>
          ${many ? `<div class="well cv-tracks"><div class="eyebrow">${esc(artist)}</div>${ctx.sibs.map((s, k) => `<button class="nav-row ${s === n ? 'cur' : ''}" data-k="${k}">${g('IconFileAudio')}<span>${esc(s.name.replace(/\.[^.]+$/, ''))}</span><span class="meta glim-num">${fmtDur(s.size / (FS.ext(s.name) === 'flac' ? 110 * KB : 40 * KB))}</span></button>`).join('')}</div>` : ''}
        </div>`;
      },
      frame: (st) => $$('.cv-wave i', host).forEach((b, k, a) => b.classList.toggle('on', k / a.length < st.t / st.dur)),
    });
    host.addEventListener('click', (e) => {
      const s = e.target.closest('[data-step]'), t = e.target.closest('[data-k]');
      const was = p.st.playing;
      const go = (m) => { ctx.go(m); if (was) setTimeout(() => $('.cv-host [data-play]')?.click(), 0); };
      if (s) go(ctx.sibs[(i + +s.dataset.step + ctx.sibs.length) % ctx.sibs.length]);
      if (t) go(ctx.sibs[+t.dataset.k]);
    });
    if (ctx.autoplay) p.toggle();
    return { ...p, info };
  };

  R.dir = (host, n, ctx) => {
    const kids = FS.kids(n).filter(k => CC.prefs.showHidden || !k.name.startsWith('.'));
    const dirs = kids.filter(k => k.dir).length, files = kids.length - dirs;
    const info = kids.length ? [dirs ? plural(dirs, 'Ordner', 'Ordner') : '', files ? plural(files, 'Datei', 'Dateien') : ''].filter(Boolean).join(', ') : 'leer';
    const pics = kids.filter(k => FS.kindOf(k) === 'img');
    if (!kids.length) host.innerHTML = `<div class="empty">${g('IconFolder')}<div>Dieser Ordner ist leer</div></div>`;
    else if (pics.length >= kids.length / 2) {
      host.innerHTML = `<div class="cv-grid">${pics.slice(0, 12).map(k => `<div class="cv-cell">${picture(k, ctx.folder)}</div>`).join('')}</div>${pics.length > 12 ? `<div class="muted cv-more">und ${plural(pics.length - 12, 'weiteres Bild', 'weitere Bilder')}</div>` : ''}`;
    } else {
      const top = kids.slice().sort((a, b) => (b.dir - a.dir) || a.name.localeCompare(b.name, 'de', { numeric: true }));
      host.innerHTML = `<div class="cv-mini">${top.slice(0, 12).map(k => `<div class="cv-mrow"><span class="tile">${g(FS.glyphOf(k))}</span><span class="t">${esc(k.name)}</span><span class="pill glim-num">${fmtSize(FS.totalSize(k)) || '0 B'}</span></div>`).join('')}</div>${top.length > 12 ? `<div class="muted cv-more">und ${plural(top.length - 12, 'weiteres Objekt', 'weitere Objekte')}</div>` : ''}`;
    }
    return { info };
  };

  CC.css(`
.cv-win .win-body { padding-bottom: 0; }
.cv-host { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: .75rem; outline: none; }
.cv-bar { display: flex; align-items: center; gap: .5rem; min-height: var(--btn-h); flex: none; }
.cv-bar .grow { flex: 1; }
.cv-bar .pill { justify-self: auto; }
.cv-stage { position: relative; flex: 1; min-height: 0; border-radius: var(--radius-card); background: var(--carbon-surface2); overflow: hidden; display: grid; place-items: center; }
.cv-stage.full { overflow: auto; display: block; cursor: grab; }
.cv-stage.full.panning { cursor: grabbing; }
.cv-box { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: .75rem; }
.cv-host, .cv-compact { contain: inline-size; }
.scrim:has(> .cv-win) { padding-bottom: 5.25rem; }
.cv-win.full { height: calc(100vh - 6.75rem); }
.cv-stage.fit { container-type: size; }
.cv-stage.fit .cv-pic { width: min(100cqw - 2rem, (100cqh - 2rem) * var(--ar)); aspect-ratio: var(--ar); border-radius: var(--radius-control); overflow: hidden; box-shadow: var(--elevation); }
.cv-stage.fit .cv-pic svg { width: 100%; height: 100%; }
.cv-stage.fit .cv-pic img { width: 100%; height: 100%; object-fit: contain; }
.cv-stage.full .cv-pic svg { width: 100%; height: 100%; }
.cv-nav { position: absolute; top: 50%; transform: translateY(-50%); width: 2.75rem; height: 2.75rem; border-radius: var(--radius-pill); background: var(--carbon-surface); box-shadow: var(--elevation), var(--hairline); display: grid; place-items: center; opacity: 0; transition: opacity .15s; z-index: 1; }
.cv-nav svg { width: 1.1rem; height: 1.1rem; }
.cv-nav:hover { background: var(--carbon-surface2); color: var(--accent-glyph); }
.cv-nav.prev { left: 1rem; } .cv-nav.next { right: 1rem; }
.cv-stage:hover .cv-nav, .cv-nav:focus-visible { opacity: 1; }
@media (hover: none) { .cv-nav { opacity: 1; } }
.cv-strip { display: flex; gap: .45rem; overflow-x: auto; padding: .2rem .1rem .4rem; flex: none; scrollbar-width: thin; }
.cv-thumb { width: 5.2rem; height: 3.9rem; flex: none; border-radius: var(--radius-control); overflow: hidden; opacity: .6; outline-offset: 2px; transition: opacity .12s; }
.cv-thumb svg, .cv-thumb img { width: 100%; height: 100%; object-fit: cover; }
.cv-thumb:hover { opacity: 1; }
.cv-thumb.cur { opacity: 1; outline: 2px solid var(--accent); }
.cv-pic-c { border-radius: var(--radius-control); overflow: hidden; aspect-ratio: 4 / 3; }
.cv-pic-c svg, .cv-pic-c img { width: 100%; height: 100%; object-fit: cover; }
.cv-compact { display: flex; flex-direction: column; gap: .75rem; flex: none; }
.cv-details .frow { min-height: 1.9rem; }
.cv-details .eyebrow { text-transform: none; letter-spacing: 0; font-size: 13px; color: var(--carbon-text); font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block; }

.cv-video { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: .7rem; }
.cv-screen { position: relative; flex: 1; min-height: 0; border-radius: var(--radius-card); overflow: hidden; background: #000; display: grid; place-items: center; cursor: pointer; }
.cv-screen > svg { position: absolute; inset: 0; width: 100%; height: 100%; transition: transform .25s linear; transform: scale(1.04); }
.cv-screen.compact { width: 100%; aspect-ratio: 16 / 9; flex: none; }
.cv-big { position: relative; width: 4rem; height: 4rem; border-radius: var(--radius-control); background: var(--accent); color: var(--accent-contrast); display: grid; place-items: center; box-shadow: 0 10px 30px rgba(0,0,0,.4); transition: opacity .2s, transform .2s; }
.cv-big svg { position: static; width: 1.5rem; height: 1.5rem; transform: none; }
.cv-screen:hover .cv-big { transform: scale(1.06); }
.playing .cv-big { opacity: 0; transform: scale(.9); }
.cv-vtitle { position: absolute; left: 0; right: 0; bottom: 0; padding: 4rem 2.2rem 1.8rem; background: linear-gradient(transparent, rgba(0,0,0,.7)); color: #fff; display: flex; flex-direction: column; text-align: left; transition: opacity .3s; pointer-events: none; }
.cv-vtitle b { font-size: clamp(22px, 4vw, 52px); line-height: 1.1; letter-spacing: -.01em; }
.cv-vtitle span { opacity: .75; font-size: clamp(13px, 1.4vw, 18px); letter-spacing: .1em; }
.playing .cv-vtitle { opacity: 0; }
.cv-sub { position: absolute; left: 50%; bottom: 7%; transform: translateX(-50%); max-width: 80%; text-align: center; color: #fff; font-size: clamp(14px, 2.2vw, 26px); font-weight: 600; text-shadow: 0 2px 6px rgba(0,0,0,.9), 0 0 2px #000; pointer-events: none; }
.cv-ctrl { display: flex; align-items: center; gap: .6rem; flex: none; }
.cv-ctrl .btn span { max-width: 12rem; overflow: hidden; text-overflow: ellipsis; }
.cv-scrub { --p: 0%; position: relative; flex: 1; height: 1.6rem; cursor: pointer; touch-action: none; }
.cv-scrub::before { content: ""; position: absolute; left: 0; right: 0; top: 50%; height: .35rem; transform: translateY(-50%); border-radius: var(--radius-pill); background: var(--carbon-surface3); }
.cv-scrub > i { position: absolute; left: 0; top: 50%; height: .35rem; width: var(--p); transform: translateY(-50%); border-radius: var(--radius-pill); background: var(--accent); }
.cv-scrub > b { position: absolute; top: 50%; left: var(--p); width: .95rem; height: .95rem; border-radius: 50%; background: var(--accent); transform: translate(-50%, -50%); box-shadow: 0 1px 4px rgba(0,0,0,.4); }
.cv-ctrl .cv-now { min-width: 3.2rem; text-align: right; }

.cv-audio { flex: 1; min-height: 0; display: grid; grid-template-columns: minmax(12rem, 20rem) 1fr; grid-template-rows: auto auto auto 1fr; gap: 1rem 2rem; align-content: start; padding: 1rem .5rem; overflow: auto; }
.cv-cover { grid-row: 1 / 4; aspect-ratio: 1; border-radius: var(--radius-card); overflow: hidden; box-shadow: var(--elevation); }
.cv-cover svg { width: 100%; height: 100%; }
.cv-atitle { align-self: end; } .cv-atitle .t { font-size: 22px; font-weight: 700; letter-spacing: -.01em; }
.cv-wave { height: 5rem; display: flex; align-items: center; gap: 1px; }
.cv-wave::before, .cv-wave > b { display: none; }
.cv-wave i { position: static; transform: none; flex: 1; min-width: 1px; width: auto; border-radius: var(--radius-pill); background: var(--carbon-surface3); }
.cv-wave i.on { background: var(--accent); }
.cv-actl { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: .75rem; }
.cv-actl > :last-child { justify-self: end; }
.cv-tr { display: flex; gap: .5rem; align-items: center; }
.btn.icon.cv-playbtn { width: 2.75rem; height: 2.75rem; border-radius: var(--radius-control); background: var(--accent); }
.cv-vbar { display: flex; align-items: center; gap: .75rem; padding: .55rem .2rem 0; font-size: 12px; }
.cv-playbtn svg { color: var(--accent-contrast); width: 1.1rem; height: 1.1rem; }
.cv-playbtn:hover { background: color-mix(in srgb, var(--accent) 85%, var(--carbon-text)); }
.cv-playbtn:hover svg { color: var(--accent-contrast); }
.cv-tracks { grid-column: 1 / -1; gap: 2px; align-self: start; }
.cv-audio.compact { grid-template-columns: 1fr; padding: 0; gap: .7rem; overflow: visible; }
.cv-audio.compact .cv-cover { grid-row: auto; width: 60%; justify-self: center; }
.cv-audio.compact .cv-atitle { text-align: center; } .cv-audio.compact .cv-atitle .t { font-size: 17px; }
.cv-audio.compact .cv-wave { height: 3rem; }

.cv-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(6.5rem, 1fr)); gap: .4rem; }
.cv-cell { aspect-ratio: 4 / 3; border-radius: var(--radius-control); overflow: hidden; }
.cv-cell svg { width: 100%; height: 100%; }
.cv-more { font-size: 12px; text-align: center; }
.cv-mini { display: flex; flex-direction: column; }
.cv-mrow { display: grid; grid-template-columns: 2.25rem 1fr auto; align-items: center; gap: .6rem; height: 2.1rem; padding: 0 .4rem; }
.cv-mrow .t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cv-mrow .tile svg { color: var(--carbon-text-sub); }

@media (max-width: 760px) {
  .scrim:has(> .cv-win) { padding: .5rem .5rem 4.75rem; }
  .cv-win.full { height: calc(100vh - 5.25rem); max-height: none; padding: 1.4rem .75rem .75rem; }
  .cv-win .win-foot { flex-wrap: wrap; }
  .cv-win .win-foot .start { flex-basis: 100%; }
  .cv-ctrl { flex-wrap: wrap; }
  .cv-ctrl .cv-scrub { flex-basis: calc(100% - 9rem); }
  .cv-ctrl [data-pick] { flex: 1; }
  .cv-audio { grid-template-columns: 1fr; padding: .25rem 0; }
  .cv-cover { grid-row: auto; width: 70%; justify-self: center; }
  .cv-strip { display: none; }
}
`);
})();
