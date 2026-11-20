import { createPinia, setActivePinia } from 'pinia';
import { createApp, nextTick, reactive } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Job, JobConflict } from '@/api/ops';
import { winStack } from '@/core/windows';
import { addMessages } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import ConflictsBody from './ConflictsBody.vue';
import { askRunning, resolveConflicts } from './conflicts';
import de from './de.json';
import en from './en.json';

addMessages('transfer', { de, en });

const side = (mtime: string) => ({ path: '', dir: false, size: 41_000_000, mtime });
const conflict = (key: string, newer?: 'source' | 'target'): JobConflict => ({ key, source: side('2026-04-16T13:18:00Z'), target: side('2026-04-16T13:18:00Z'), ...(newer ? { newer } : {}) });
const top = () => winStack.at(-1)!;
const buttons = () => top().foot.filter((f) => f.label).map((f) => f.label);
function press(label: string) {
  const w = top(), f = w.foot.find((b) => b.label === label)!;
  if (!f.run || f.run(w.el!) !== false) w.close();
}

beforeEach(() => {
  setActivePinia(createPinia());
  winStack.splice(0);
});

describe('the conflict window', () => {
  it('answers a single name with one of the buttons in its foot', async () => {
    const answer = resolveConflicts([conflict('album.flac')], '/mnt/user/musik');
    expect(buttons()).toEqual(['Abbrechen', 'Ersetzen', 'Überspringen', 'Beide behalten']);
    expect(top().foot.find((f) => f.primary)?.label).toBe('Beide behalten');
    press('Ersetzen');
    expect(await answer).toEqual({ 'album.flac': 'replace' });
    expect(winStack).toHaveLength(0);
  });

  it('goes on to the next name after each answer', async () => {
    const answer = resolveConflicts([conflict('a.mkv'), conflict('b.mkv'), conflict('c.mkv')], '/mnt/user/filme');
    press('Überspringen');
    expect(winStack).toHaveLength(1);
    expect((top().props!.st as { at: number }).at).toBe(1);
    press('Ersetzen');
    press('Beide behalten');
    expect(await answer).toEqual({ 'a.mkv': 'skip', 'b.mkv': 'replace', 'c.mkv': 'both' });
  });

  it('answers every name left at once when Für alle übernehmen is ticked', async () => {
    const answer = resolveConflicts([conflict('a.mkv'), conflict('b.mkv', 'source'), conflict('c.mkv', 'target')], '/mnt/user/filme');
    press('Überspringen');
    expect(buttons()).not.toContain('Nur Neuere ersetzen');
    (top().props!.st as { all: boolean }).all = true;
    await nextTick();
    expect(buttons()).toContain('Nur Neuere ersetzen');
    press('Nur Neuere ersetzen');
    expect(await answer).toEqual({ 'a.mkv': 'skip', 'b.mkv': 'replace', 'c.mkv': 'skip' });
    expect(winStack).toHaveLength(0);
  });

  it('answers nothing when it is left', async () => {
    const answer = resolveConflicts([conflict('a.mkv')], '/mnt/user/filme');
    press('Abbrechen');
    expect(await answer).toBeNull();
  });

  it('tells a waiting job the answer for its name and, when ticked, for all after it', async () => {
    const decide = vi.spyOn(useJobs(), 'decide').mockResolvedValue();
    const job = { id: 'j1', dest: '/mnt/user/filme' } as Job;
    askRunning(job, conflict('b.mkv', 'target'));
    expect(buttons()).not.toContain('Nur Neuere ersetzen');
    (top().props!.st as { all: boolean }).all = true;
    await nextTick();
    expect(buttons()).toContain('Nur Neuere ersetzen');
    press('Ersetzen');
    expect(decide).toHaveBeenCalledWith(job, 'b.mkv', 'replace', true);
  });
});

describe('the body of the conflict window', () => {
  let app: ReturnType<typeof createApp> | undefined;
  afterEach(() => {
    app?.unmount();
    document.body.innerHTML = '';
  });
  const mount = (conflicts: JobConflict[], at: number, later = false) => {
    app = createApp(ConflictsBody, { conflicts, dest: '/mnt/user/filme', st: reactive({ at, all: false }), later });
    app.mount(document.body.appendChild(document.createElement('div')));
  };

  it('shows the name it asks about and how far along it is', () => {
    mount([conflict('a.mkv'), conflict('b.mkv'), conflict('c.mkv')], 1);
    expect(document.querySelector('.cf-name b')?.textContent).toBe('b.mkv');
    expect(document.body.textContent).toContain('2 von 3');
    expect(document.querySelectorAll('.cf-row')).toHaveLength(1);
  });

  it('offers Für alle übernehmen only while more than one name is left', () => {
    mount([conflict('a.mkv'), conflict('b.mkv')], 0);
    expect(document.querySelector('[role="checkbox"]')?.textContent).toContain('Für alle übernehmen');
    app!.unmount();
    mount([conflict('a.mkv'), conflict('b.mkv')], 1);
    expect(document.querySelector('[role="checkbox"]')).toBeNull();
  });

  it('offers Für alle übernehmen for the one name a running job asks about', () => {
    mount([conflict('a.mkv')], 0, true);
    expect(document.querySelector('[role="checkbox"]')).not.toBeNull();
  });
});
