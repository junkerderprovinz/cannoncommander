// Passkey options and answers travel as JSON with base64url fields, while the browser's credential calls want
// buffers. Browsers that convert by themselves do it, the others get the same done here.
type Json = Record<string, unknown>;
type Ctor = typeof PublicKeyCredential & {
  parseCreationOptionsFromJSON?: (o: unknown) => PublicKeyCredentialCreationOptions;
  parseRequestOptionsFromJSON?: (o: unknown) => PublicKeyCredentialRequestOptions;
};

export function fromB64url(s: string): ArrayBuffer {
  const b = atob(s.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (s.length % 4)) % 4));
  return Uint8Array.from(b, (c) => c.charCodeAt(0)).buffer;
}

export function toB64url(buf: ArrayBuffer | ArrayBufferView): string {
  const bytes = buf instanceof ArrayBuffer ? new Uint8Array(buf) : new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength);
  let s = '';
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

const ctor = () => (typeof PublicKeyCredential === 'undefined' ? undefined : (PublicKeyCredential as Ctor));
// Browsers offer passkeys only over HTTPS and on localhost, elsewhere PublicKeyCredential is missing.
export const browserHasPasskeys = () => ctor() !== undefined;
const withIds = (list: unknown) => (Array.isArray(list) ? list.map((c: Json) => ({ ...c, id: fromB64url(c.id as string) })) : list);

export function creationOptions(options: { publicKey: Json }): CredentialCreationOptions {
  const pk = options.publicKey, native = ctor()?.parseCreationOptionsFromJSON;
  if (native) return { publicKey: native(pk) };
  const user = pk.user as Json;
  return { publicKey: { ...pk, challenge: fromB64url(pk.challenge as string), user: { ...user, id: fromB64url(user.id as string) }, excludeCredentials: withIds(pk.excludeCredentials) } as unknown as PublicKeyCredentialCreationOptions };
}

export function requestOptions(options: { publicKey: Json }): CredentialRequestOptions {
  const pk = options.publicKey, native = ctor()?.parseRequestOptionsFromJSON;
  if (native) return { publicKey: native(pk) };
  return { publicKey: { ...pk, challenge: fromB64url(pk.challenge as string), allowCredentials: withIds(pk.allowCredentials) } as unknown as PublicKeyCredentialRequestOptions };
}

// The answer as the server reads it.
export function credentialJSON(c: Credential | null): unknown {
  if (!c) throw new Error('no credential');
  const pk = c as PublicKeyCredential & { toJSON?: () => unknown };
  if (typeof pk.toJSON === 'function') return pk.toJSON();
  const r = pk.response as AuthenticatorAttestationResponse & AuthenticatorAssertionResponse;
  const response: Json = { clientDataJSON: toB64url(r.clientDataJSON) };
  if ('attestationObject' in r && r.attestationObject) response.attestationObject = toB64url(r.attestationObject);
  if ('authenticatorData' in r && r.authenticatorData) {
    response.authenticatorData = toB64url(r.authenticatorData);
    response.signature = toB64url(r.signature);
    if (r.userHandle) response.userHandle = toB64url(r.userHandle);
  }
  return { id: pk.id, rawId: toB64url(pk.rawId), type: pk.type, response, clientExtensionResults: pk.getClientExtensionResults?.() ?? {} };
}
