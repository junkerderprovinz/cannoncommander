import { describe, expect, it } from 'vitest';
import { CLOUD_CLIENTS, clientById, LOCAL_CLIENTS, OTHER_CLIENT, snippetFor } from './clients';
import { KEY_FILE_PLACEHOLDER, KEY_VARIABLE, mcpUrl, SNIPPETS } from './snippets';

const input = { origin: 'http://tower.lan:8080/', endpointPath: 'mcp', key: 'ccmcp_secret' };

describe('the client snippets', () => {
  it('join the address and the endpoint with one slash', () => {
    expect(mcpUrl(input)).toBe('http://tower.lan:8080/mcp');
    expect(mcpUrl({ origin: 'https://files.example.com/cc/', endpointPath: '/mcp' })).toBe('https://files.example.com/cc/mcp');
  });

  it('give every client but the OAuth ones something to copy', () => {
    for (const c of [...LOCAL_CLIENTS, ...CLOUD_CLIENTS, OTHER_CLIENT]) {
      expect(!!snippetFor(c, input), c.id).toBe(!c.oauth);
    }
  });

  it('never put the key on a command line', () => {
    for (const id of ['claude-code', 'claude-desktop', 'perplexity'] as const) {
      const text = SNIPPETS[id](input);
      expect(text).not.toContain(input.key);
      expect(text).toContain(KEY_FILE_PLACEHOLDER);
      expect(text).toContain('--allow-http');
    }
    expect(SNIPPETS['claude-code']({ ...input, origin: 'https://files.example.com' })).not.toContain('--allow-http');
  });

  it('name the variable where a client reads the key from the environment', () => {
    for (const c of LOCAL_CLIENTS.filter((x) => x.key.kind === 'env')) {
      const text = snippetFor(c, input)!;
      expect(text, c.id).toContain(KEY_VARIABLE);
      expect(text, c.id).not.toContain(input.key);
    }
  });

  it('hold the key where the client cannot read it from anywhere else', () => {
    for (const c of LOCAL_CLIENTS.filter((x) => x.key.kind === 'inFile' || x.key.kind === 'inApp')) {
      expect(snippetFor(c, input), c.id).toContain(input.key);
    }
  });

  it('keep the client ids apart and find a key\'s client again', () => {
    const ids = [...LOCAL_CLIENTS, ...CLOUD_CLIENTS].map((c) => c.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(clientById('cursor')?.name).toBe('Cursor');
    expect(clientById('claude')?.oauth).toBe('claude');
    expect(clientById('other')).toBeUndefined();
    expect(clientById('')).toBeUndefined();
  });
});
