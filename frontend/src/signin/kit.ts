// The kit's sign-in as CannonCommander talks to it: one client for the page, the tile and the password question.
import { createSignInApi, type SignInReauth, type SignInReauthState } from '@glimstone/core';
import { ApiError } from '@/api/client';

export const signIn = createSignInApi({ root: new URL(import.meta.env.BASE_URL, location.href).pathname });

// The kit's client asks for the password only before its own changes. CannonCommander's API wants a fresh sign-in
// too, for a password an admin sets and for the MCP server's keys, so one question serves both and SignInReauthWindow
// draws whichever is waiting.
function shareQuestion(kit: SignInReauth) {
  let own: SignInReauthState = { asking: false, password: '', busy: false, error: null, refused: 0 };
  const listeners = new Set<() => void>();
  let waiting: ((confirmed: boolean) => void) | null = null;
  const update = (change: Partial<SignInReauthState>) => {
    own = { ...own, ...change };
    listeners.forEach((fn) => fn());
  };
  const settle = (confirmed: boolean) => {
    const resume = waiting;
    waiting = null;
    update({ asking: false, password: '', busy: false, error: null });
    resume?.(confirmed);
  };
  const kitAsks = () => kit.state().asking;

  const reauth: SignInReauth = {
    state: () => (kitAsks() ? kit.state() : own),
    subscribe(listener) {
      listeners.add(listener);
      const stop = kit.subscribe(listener);
      return () => {
        listeners.delete(listener);
        stop();
      };
    },
    set: (password) => (kitAsks() ? kit.set(password) : update({ password, error: null })),
    async submit() {
      if (kitAsks()) return kit.submit();
      if (!own.asking || own.busy || own.password === '') return;
      update({ busy: true, error: null });
      const r = await signIn.confirm(own.password);
      if (r.ok) settle(true);
      else update({ busy: false, error: r.error, refused: own.refused + 1 });
    },
    cancel: () => (kitAsks() ? kit.cancel() : settle(false)),
  };

  let pending: Promise<boolean> | null = null;
  const ask = (): Promise<boolean> => {
    if (listeners.size === 0) return Promise.resolve(false);
    pending ??= new Promise<boolean>((resolve) => {
      waiting = resolve;
      update({ asking: true, password: '', error: null });
    }).finally(() => (pending = null));
    return pending;
  };
  return { reauth, ask };
}

const question = shareQuestion(signIn.reauth);

export const reauthClient = { ...signIn, reauth: question.reauth };

const wantsPassword = (e: unknown) => e instanceof ApiError && e.problem?.code === 'signInAgain' && e.problem.params?.askPassword === true;

// Runs a change of CannonCommander's API, and once more after the password when it wants a fresh sign-in.
export async function withPassword<T>(change: () => Promise<T>): Promise<T> {
  try {
    return await change();
  } catch (e) {
    if (!wantsPassword(e) || !(await question.ask())) throw e;
    return change();
  }
}
