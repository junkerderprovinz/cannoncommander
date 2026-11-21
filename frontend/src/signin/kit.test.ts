import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@/api/client';
import { reauthClient, withPassword } from './kit';

const stale = (askPassword: boolean) => new ApiError(403, { type: '', title: 'Forbidden', status: 403, code: 'signInAgain', params: { askPassword } });
let confirmed: unknown[] = [];
let confirmAnswer = { status: 200, body: {} as unknown };

beforeEach(() => {
  confirmed = [];
  confirmAnswer = { status: 200, body: {} };
  vi.stubGlobal('fetch', async (url: string, init: RequestInit = {}) => {
    if (url.endsWith('api/signin/v1/me/confirm')) confirmed.push(JSON.parse(String(init.body)));
    return new Response(JSON.stringify(confirmAnswer.body), { status: confirmAnswer.status, headers: { 'Content-Type': 'application/json' } });
  });
});
afterEach(() => vi.unstubAllGlobals());

const tick = () => new Promise((r) => setTimeout(r, 0));
const question = reauthClient.reauth;
function listen() {
  return question.subscribe(() => {});
}

describe('a change that wants a fresh sign-in', () => {
  it('goes again once the password is confirmed', async () => {
    const stop = listen();
    const change = vi.fn().mockRejectedValueOnce(stale(true)).mockResolvedValueOnce('done');
    const result = withPassword(change);
    await tick();
    expect(question.state().asking).toBe(true);
    question.set('mein-passwort');
    await question.submit();
    expect(await result).toBe('done');
    expect(confirmed).toEqual([{ password: 'mein-passwort' }]);
    expect(change).toHaveBeenCalledTimes(2);
    expect(question.state().asking).toBe(false);
    stop();
  });

  it('keeps asking after a wrong password and gives up on Cancel', async () => {
    const stop = listen();
    confirmAnswer = { status: 401, body: { error: 'wrongPassword' } };
    const refusal = stale(true);
    const change = vi.fn().mockRejectedValue(refusal);
    const result = withPassword(change);
    await tick();
    question.set('falsch');
    await question.submit();
    expect(question.state()).toMatchObject({ asking: true, error: 'wrongPassword', refused: 1 });
    question.cancel();
    await expect(result).rejects.toBe(refusal);
    expect(change).toHaveBeenCalledTimes(1);
    stop();
  });

  it('asks once for two changes that wait at the same time', async () => {
    const stop = listen();
    const a = vi.fn().mockRejectedValueOnce(stale(true)).mockResolvedValueOnce('a');
    const b = vi.fn().mockRejectedValueOnce(stale(true)).mockResolvedValueOnce('b');
    const both = Promise.all([withPassword(a), withPassword(b)]);
    await tick();
    question.set('mein-passwort');
    await question.submit();
    expect(await both).toEqual(['a', 'b']);
    expect(confirmed).toHaveLength(1);
    stop();
  });

  it('passes the refusal on where the account has no password, or nothing on the page can ask', async () => {
    const stop = listen();
    const noPassword = stale(false);
    await expect(withPassword(() => Promise.reject(noPassword))).rejects.toBe(noPassword);
    stop();
    const nobody = stale(true);
    await expect(withPassword(() => Promise.reject(nobody))).rejects.toBe(nobody);
    expect(confirmed).toEqual([]);
  });

  it('leaves every other refusal alone', async () => {
    const stop = listen();
    const other = new ApiError(403, { type: '', title: 'Forbidden', status: 403, code: 'ownBlock' });
    await expect(withPassword(() => Promise.reject(other))).rejects.toBe(other);
    expect(question.state().asking).toBe(false);
    stop();
  });
});
