import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick } from 'vue';
import { lazyComponent, twice } from './lazy';

const Part = defineComponent({ props: { name: String }, setup: (props) => () => h('p', { class: 'part' }, `Hallo ${props.name}`) });
const gone = () => Promise.reject(new TypeError('error loading dynamically imported module'));

const settle = async () => {
  for (let i = 0; i < 4; i++) {
    await new Promise((r) => setTimeout(r, 0));
    await nextTick();
  }
};
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});
async function mount(load: () => Promise<{ default: typeof Part }>) {
  const Lazy = lazyComponent(load);
  app = createApp({ render: () => h(Lazy, { name: 'Anna' }) });
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}

describe('a part that comes in a chunk of its own', () => {
  it('tries a second time before it gives up', async () => {
    const load = vi.fn().mockImplementationOnce(gone).mockResolvedValue({ default: Part });
    await mount(load);
    expect(load).toHaveBeenCalledTimes(2);
    expect(document.querySelector('.part')?.textContent).toBe('Hallo Anna');
  });

  it('says so in its place when both tries fail, and loads on Try again', async () => {
    const load = vi.fn().mockImplementationOnce(gone).mockImplementationOnce(gone).mockResolvedValue({ default: Part });
    await mount(load);
    expect(document.querySelector('.lazy-failed')?.textContent).toContain('Dieser Teil ließ sich nicht laden.');
    expect(document.querySelector('.part')).toBeNull();
    document.querySelector<HTMLButtonElement>('[data-lazy="retry"]')!.click();
    await settle();
    expect(document.querySelector('.lazy-failed')).toBeNull();
    expect(document.querySelector('.part')?.textContent).toBe('Hallo Anna');
  });

  it('leaves nothing for the page to catch when a load fails', async () => {
    const errors: unknown[] = [];
    const record = (e: PromiseRejectionEvent) => errors.push(e.reason);
    window.addEventListener('unhandledrejection', record);
    await mount(() => gone());
    window.removeEventListener('unhandledrejection', record);
    expect(errors).toEqual([]);
  });

  it('hands a plain load its second try as well', async () => {
    const load = vi.fn().mockImplementationOnce(gone).mockResolvedValue('arrived');
    expect(await twice(load)()).toBe('arrived');
    await expect(twice(gone)()).rejects.toThrow('dynamically imported');
  });
});
