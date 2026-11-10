// The way in and out: password, the second step, passkeys and the reset with a code from the container log.
import { call, json } from './client';
import type { Me } from './types';
import { credentialJSON, requestOptions } from './webauthn';

export interface SignInOptions { password: boolean; passkey: boolean; oidc: { label: string; url: string } | null; server: string; version: string; minPassword: number }
export type Step = 'done' | 'totp' | 'password' | 'totpSetup';
export interface TotpSetup { secret: string; uri: string; qr: string }
export interface SignIn { next: Step; challenge?: string; me?: Me; setup?: TotpSetup; codes?: string[] }

const post = async <T>(path: string, body?: unknown) => (await call<T>(path, body === undefined ? { method: 'POST' } : json('POST', body))).body as T;

export const auth = {
  options: async () => (await call<SignInOptions>('/auth/options')).body!,
  // A sign in from the page always stays for the days chosen under Security.
  login: (username: string, password: string) => post<SignIn>('/auth/login', { username, password, remember: true }),
  // A code from the app or a recovery code, also the first code of a setup the sign in asked for.
  code: (challenge: string, code: string) => post<SignIn>('/auth/totp', { challenge, code }),
  newPassword: (challenge: string, password: string) => post<SignIn>('/auth/password', { challenge, password }),
  async passkey(): Promise<SignIn> {
    const { challenge, options } = await post<{ challenge: string; options: { publicKey: Record<string, unknown> } }>('/auth/passkey/options');
    const credential = credentialJSON(await navigator.credentials.get(requestOptions(options)));
    return post<SignIn>('/auth/passkey', { challenge, credential, remember: true });
  },
  logout: () => post<void>('/auth/logout'),
  askReset: (username: string) => post<void>('/auth/reset-code', { username }),
  reset: (code: string, password: string) => post<Me>('/auth/reset', { code, password }),
};
