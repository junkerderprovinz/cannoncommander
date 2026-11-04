import { describe, expect, it } from 'vitest';
import { credentialJSON, fromB64url, requestOptions, toB64url } from './webauthn';

describe('passkey wire format', () => {
  it('turns base64url into bytes and back without padding', () => {
    const bytes = new Uint8Array([0, 251, 255, 62, 63, 1]);
    const text = toB64url(bytes);
    expect(text).toBe('APv_Pj8B');
    expect(new Uint8Array(fromB64url(text))).toEqual(bytes);
  });

  it('gives the browser buffers where the server sent text', () => {
    const got = requestOptions({ publicKey: { challenge: 'AQID', rpId: 'cannon.lan', allowCredentials: [{ type: 'public-key', id: 'BAU' }] } }).publicKey!;
    expect(new Uint8Array(got.challenge as ArrayBuffer)).toEqual(new Uint8Array([1, 2, 3]));
    expect(new Uint8Array(got.allowCredentials![0]!.id as ArrayBuffer)).toEqual(new Uint8Array([4, 5]));
    expect(got.rpId).toBe('cannon.lan');
  });

  it('writes an answer the server parses when the browser has no toJSON', () => {
    const buf = (...b: number[]) => new Uint8Array(b).buffer;
    const answer = credentialJSON({
      id: 'Bwg', rawId: buf(7, 8), type: 'public-key', getClientExtensionResults: () => ({}),
      response: { clientDataJSON: buf(1), authenticatorData: buf(2), signature: buf(3), userHandle: buf(52, 50) },
    } as unknown as Credential);
    expect(answer).toEqual({
      id: 'Bwg', rawId: 'Bwg', type: 'public-key', clientExtensionResults: {},
      response: { clientDataJSON: 'AQ', authenticatorData: 'Ag', signature: 'Aw', userHandle: 'NDI' },
    });
  });
});
