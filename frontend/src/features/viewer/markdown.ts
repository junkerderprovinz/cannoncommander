// A small markdown renderer, enough for notes, task lists, tables and code, and a quiet highlight for code:
// comments step back, strings and keys a little. Both escape the text first, so a file can never inject markup.
import { glyph } from '@/core/glyphs';

export const esc = (s: string) => s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!);

export const glyphSvg = (name: string) => {
  const d = glyph(name);
  return `<svg viewBox="${d.viewBox}" fill="currentColor" aria-hidden="true">${d.svg}</svg>`;
};

// Only web links become links; anything else, such as javascript:, stays text.
const safeHref = (href: string) => (/^(https?:|mailto:|#|\/)/i.test(href) ? href : '');

function inline(s: string) {
  return esc(s)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (all, text: string, href: string) => {
      const to = safeHref(href.replace(/&amp;/g, '&'));
      return to ? `<a href="${esc(to)}" target="_blank" rel="noopener">${text}</a>` : text;
    });
}

const LIST = /^\s*([-*]|\d+\.)\s+(.*)/;

export function md(src: string): string {
  const out: string[] = [];
  const L = src.replace(/\r/g, '').split('\n');
  for (let i = 0; i < L.length; i++) {
    const l = L[i]!;
    if (/^```/.test(l)) {
      const buf: string[] = [];
      while (++i < L.length && !/^```/.test(L[i]!)) buf.push(L[i]!);
      out.push(`<pre><code>${esc(buf.join('\n'))}</code></pre>`);
      continue;
    }
    const h = l.match(/^(#{1,4})\s+(.*)/);
    if (h) { out.push(`<h${h[1]!.length}>${inline(h[2]!)}</h${h[1]!.length}>`); continue; }
    if (/^\s*(-{3,}|\*{3,})\s*$/.test(l)) { out.push('<hr>'); continue; }
    if (/^>\s?/.test(l)) {
      const buf: string[] = [];
      while (i < L.length && /^>\s?/.test(L[i]!)) buf.push(L[i++]!.replace(/^>\s?/, ''));
      i--;
      out.push(`<blockquote>${inline(buf.join(' '))}</blockquote>`);
      continue;
    }
    if (/^\|/.test(l)) {
      const rows: string[] = [];
      while (i < L.length && /^\|/.test(L[i]!)) rows.push(L[i++]!);
      i--;
      const cells = (r: string) => r.replace(/^\||\|$/g, '').split('|').map((c) => c.trim());
      const body = rows.filter((r, k) => k !== 1 || !/^[\s|:-]+$/.test(r));
      out.push(`<table><thead><tr>${cells(body[0]!).map((c) => `<th>${inline(c)}</th>`).join('')}</tr></thead><tbody>${body.slice(1).map((r) => `<tr>${cells(r).map((c) => `<td>${inline(c)}</td>`).join('')}</tr>`).join('')}</tbody></table>`);
      continue;
    }
    const li = l.match(LIST);
    if (li) {
      const ordered = /\d/.test(li[1]!), items: string[] = [];
      while (i < L.length) {
        const m = L[i]!.match(LIST);
        if (!m) break;
        items.push(m[2]!);
        i++;
      }
      i--;
      out.push(`<${ordered ? 'ol' : 'ul'}>${items.map((it) => {
        const task = it.match(/^\[( |x)\]\s+(.*)/i);
        return task
          ? `<li class="task ${task[1] !== ' ' ? 'done' : ''}"><span class="mark">${task[1] !== ' ' ? glyphSvg('IconCheck') : ''}</span>${inline(task[2]!)}</li>`
          : `<li>${inline(it)}</li>`;
      }).join('')}</${ordered ? 'ol' : 'ul'}>`);
      continue;
    }
    if (!l.trim()) continue;
    const para = [l];
    while (i + 1 < L.length && L[i + 1]!.trim() && !/^(#|>|\||```|\s*([-*]|\d+\.)\s)/.test(L[i + 1]!)) para.push(L[++i]!);
    out.push(`<p>${inline(para.join(' '))}</p>`);
  }
  return out.join('');
}

const HIGHLIT = ['json', 'xml', 'conf', 'cfg', 'yml', 'yaml'];

export function highlight(text: string, kind: string, e: string): string {
  const s = esc(text);
  if (kind !== 'code' && !HIGHLIT.includes(e)) return s;
  if (e === 'go') {
    return s.replace(/(\/\/.*$)|(&quot;[^&]*?&quot;|"[^"\n]*")|\b(package|import|func|var|const|return|if|else|for|range|type|struct|err|nil)\b/gm,
      (m, c, str, k) => (c ? `<i class="c">${c}</i>` : str ? `<i class="s">${str}</i>` : `<i class="k">${k}</i>`));
  }
  if (e === 'yml' || e === 'yaml') {
    return s.replace(/((?<!&)#.*$)|^(\s*-?\s*)([\w.-]+)(:)|(&quot;.*?&quot;|"[^"\n]*")/gm,
      (m, c, pre, key, col, str) => (c ? `<i class="c">${c}</i>` : str ? `<i class="s">${str}</i>` : `${pre}<i class="k">${key}</i>${col}`));
  }
  if (e === 'json') return s.replace(/(&quot;[^&]*?&quot;)(\s*:)?/g, (m, str, col) => (col ? `<i class="k">${str}</i>${col}` : `<i class="s">${str}</i>`));
  // One pass, so whichever of a string and a comment starts first wins and the markup put in is never matched again.
  // The # of an entity such as &#39; starts no comment.
  return s.replace(/(&quot;.*?&quot;)|((?<!&)#.*$)/gm, (m, str, c) => (str ? `<i class="s">${str}</i>` : `<i class="c">${c}</i>`));
}
