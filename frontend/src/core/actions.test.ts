import { beforeEach, describe, expect, it, vi } from 'vitest';
import { action, actions, blocked, labelOf, run, setActionContext } from './actions';
import { addTo } from './extend';
import { prefs } from './prefs';
import { toasts } from './toast';

const select = (sel: { name: string; dir?: boolean; at?: string }[], here = '/mnt/user') => setActionContext({ sel: () => sel, here: () => here });

beforeEach(() => {
  for (const id of Object.keys(actions)) delete actions[id];
  toasts.splice(0);
  prefs.dual = true;
  select([]);
});

describe('action registry', () => {
  it('keeps the id beside the definition', () => {
    action('copy', { label: 'Kopieren', g: 'IconCopy', key: 'F5', run: () => {} });
    expect(actions.copy).toMatchObject({ id: 'copy', label: 'Kopieren', key: 'F5' });
  });

  it('reads a label given as a function each time', () => {
    let lang = 'de';
    action('x', { label: () => (lang === 'de' ? 'Kopieren' : 'Copy'), run: () => {} });
    expect(labelOf(actions.x!)).toBe('Kopieren');
    lang = 'en';
    expect(labelOf(actions.x!)).toBe('Copy');
  });

  it('runs an action with its arguments and returns what it returns', () => {
    action('sum', { label: 'Summe', run: ((a: number, b: number) => a + b) as never });
    expect(run('sum', 2, 3)).toBe(5);
  });

  it('says so instead of failing when an action is not built yet', () => {
    run('palette');
    expect(toasts.at(-1)).toMatchObject({ text: 'Noch nicht gebaut: palette', kind: 'warn' });
  });
});

describe('what an action needs', () => {
  it('asks for a selection first', () => {
    const fn = vi.fn();
    action('rename', { label: 'Umbenennen', needs: ['sel', 'writable'], run: fn });
    run('rename');
    expect(fn).not.toHaveBeenCalled();
    expect(toasts.at(-1)?.text).toBe('Erst eine Datei oder einen Ordner wählen');
  });

  it('asks for a file when only folders are chosen', () => {
    select([{ name: 'filme', dir: true }]);
    expect(blocked({ id: 'view', needs: 'file' })).toBe('Erst eine Datei wählen');
    select([{ name: 'a.mkv' }]);
    expect(blocked({ id: 'view', needs: 'file' })).toBeNull();
  });

  it('wants both panels for a two-panel action', () => {
    prefs.dual = false;
    expect(blocked({ id: 'swapPanels', needs: 'dual' })).toBe('Dafür beide Fenster einschalten (Strg+2)');
  });

  it('only lets copy, move and delete work on hits from other folders', () => {
    select([{ name: 'a.mkv', at: '/mnt/user/filme' }]);
    expect(blocked({ id: 'rename', needs: 'sel' })).toBe('Das geht nur im eigenen Ordner: mit Enter zur Datei springen');
    expect(blocked({ id: 'copy', needs: 'sel' })).toBeNull();
  });

  it('keeps writes out of archives', () => {
    select([{ name: 'IMG_0001.jpg' }], '/mnt/user/downloads/Fotos.zip::');
    expect(blocked({ id: 'delete', needs: ['sel', 'writable'] })).toBe('Im Archiv geht nur Ansehen und Entpacken');
  });
});

describe('action wrappers', () => {
  it('lets a feature wrap every run and take itself out again', () => {
    const seen: string[] = [];
    action('refresh', { label: 'Aktualisieren', run: () => seen.push('run') });
    const off = addTo('actionWrappers', (a, inner) => (...args: never[]) => {
      seen.push('before ' + a.id);
      return inner(...args);
    });
    run('refresh');
    off();
    run('refresh');
    expect(seen).toEqual(['before refresh', 'run', 'run']);
  });
});
