import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import { ops } from '@/api/ops';
import { actions } from '@/core/actions';
import { menuState } from '@/core/menu';
import { winStack } from '@/core/windows';
import { loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import type { Entry } from '@/files/entries';
import { rowMenu } from '@/files/menus';
import { usePanels } from '@/stores/panels';

const files = [{ name: 'a.jpg', size: 2048 }, { name: 'b.jpg', size: 1024 }] as Entry[];

beforeAll(() => {
  setActivePinia(createPinia());
  usePanels();
  registerFileActions();
  loadFeatures();
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
  winStack.splice(0);
  vi.restoreAllMocks();
});

async function open(id: string) {
  vi.spyOn(usePanels(), 'sel').mockReturnValue(files);
  vi.spyOn(ops, 'preflight').mockResolvedValue({ conflicts: [], bytes: 3072 } as never);
  await actions[id]!.run();
  const w = winStack.at(-1)!;
  app = createApp({ render: () => h(w.body as never, w.props) });
  app.mount(document.body.appendChild(document.createElement('div')));
  for (let i = 0; i < 4; i++) await nextTick();
  return w;
}

describe('the copy window', () => {
  it('asks what to copy and where, and nothing about names that exist or how to copy', async () => {
    const w = await open('copy');
    expect(document.body.textContent).toContain('Kopieren?');
    expect(document.querySelector('#clash, #how')).toBeNull();
    expect(w.props!.st).not.toHaveProperty('way');
    expect(w.foot.filter((f) => f.label).map((f) => f.label)).toEqual(['Abbrechen', 'Kopieren']);
  });

  it('asks the same for a move', async () => {
    await open('move');
    expect(document.body.textContent).toContain('Verschieben?');
  });
});

describe('a hard link', () => {
  it('has its own row in the right-click menu', () => {
    vi.spyOn(usePanels(), 'sel').mockReturnValue(files);
    rowMenu(10, 10);
    expect((menuState.list ?? []).some((e) => typeof e === 'object' && 'id' in e && e.id === 'hardlink')).toBe(true);
  });

  it('opens its own window that says what a hard link is', async () => {
    const w = await open('hardlink');
    expect(w.title).toBe('Hart verlinken');
    expect(document.body.textContent).toContain('zweiter Name');
    expect(w.foot.find((f) => f.primary)?.label).toBe('Als harten Link anlegen');
  });
});
