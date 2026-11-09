// The configuration each MCP client needs, as text to copy. Every line is a command, a header, a form field or a key
// that a client parses literally, so none of it is translated.
//
// A key never goes on a command line, where the process list and the shell history would keep it. Where a client can
// take the key from a file, an environment variable or a secret prompt, its snippet names that instead; where it
// cannot, the key sits in the snippet and the setup window says so.

export interface SnippetInput {
  // Scheme, host and port of the address CannonCommander was opened at, with the base path.
  origin: string;
  endpointPath: string;
  // The fresh key, or KEY_PLACEHOLDER while there is none to show.
  key: string;
}

export const KEY_PLACEHOLDER = '<your key>';
export const KEY_FILE_PLACEHOLDER = '<path of the file with your key>';
export const KEY_VARIABLE = 'CANNONCOMMANDER_MCP_KEY';
const NAME = 'cannoncommander';

export const mcpUrl = (i: Pick<SnippetInput, 'origin' | 'endpointPath'>) => i.origin.replace(/\/+$/, '') + '/' + i.endpointPath.replace(/^\/+/, '');

const json = (value: unknown) => JSON.stringify(value, null, 2);
const plainHttp = (i: SnippetInput) => i.origin.toLowerCase().startsWith('http:');

// mcp-remote reads the key from a file, so neither the command nor the process list shows it. @latest keeps an older
// global mcp-remote without --header-file from being picked, and the path is quoted because a space would split it.
function mcpRemote(i: SnippetInput) {
  return `npx -y mcp-remote@latest ${mcpUrl(i)} --header-file "${KEY_FILE_PLACEHOLDER}"${plainHttp(i) ? ' --allow-http' : ''}`;
}

// The fields any Streamable HTTP client asks for, with both header forms.
export function genericSnippet(i: SnippetInput) {
  return [`URL: ${mcpUrl(i)}`, 'Transport: Streamable HTTP', `Authorization: Bearer ${i.key}`, `X-API-Key: ${i.key}`].join('\n');
}

const bearer = (i: SnippetInput) => ({ Authorization: `Bearer ${i.key}` });
const bearerFrom = (variable: string) => ({ Authorization: `Bearer ${variable}` });
const form = (fields: [string, string][]) => fields.map(([name, value]) => `${name}: ${value}`).join('\n');
const servers = (entry: unknown) => json({ mcpServers: { [NAME]: entry } });

// Each client's snippet under its id in the client list, shaped as its own documentation shows it.
export const SNIPPETS = {
  anythingllm: (i: SnippetInput) => servers({ type: 'streamable', url: mcpUrl(i), headers: bearer(i) }),
  antigravity: (i: SnippetInput) => servers({ serverUrl: mcpUrl(i), headers: bearerFrom(`\${${KEY_VARIABLE}}`) }),
  'claude-code': (i: SnippetInput) => `claude mcp add ${NAME} --scope user -- ${mcpRemote(i)}`,
  'claude-desktop': (i: SnippetInput) => {
    const args = ['-y', 'mcp-remote@latest', mcpUrl(i), '--header-file', KEY_FILE_PLACEHOLDER];
    if (plainHttp(i)) args.push('--allow-http');
    return servers({ command: 'npx', args });
  },
  cline: (i: SnippetInput) => servers({ type: 'streamableHttp', url: mcpUrl(i), headers: bearer(i), disabled: false }),
  codex: (i: SnippetInput) => [`[mcp_servers.${NAME}]`, `url = "${mcpUrl(i)}"`, `bearer_token_env_var = "${KEY_VARIABLE}"`].join('\n'),
  continue: (i: SnippetInput) => [
    'mcpServers:', `  - name: ${NAME}`, '    type: streamable-http', `    url: ${mcpUrl(i)}`, '    requestOptions:', '      headers:',
    `        Authorization: "Bearer \${{ secrets.${KEY_VARIABLE} }}"`,
  ].join('\n'),
  'copilot-cli': (i: SnippetInput) => servers({ type: 'http', url: mcpUrl(i), headers: bearer(i), tools: ['*'] }),
  cursor: (i: SnippetInput) => servers({ url: mcpUrl(i), headers: bearerFrom(`\${env:${KEY_VARIABLE}}`) }),
  gemini: (i: SnippetInput) => servers({ httpUrl: mcpUrl(i), headers: bearerFrom(`\${${KEY_VARIABLE}}`) }),
  copilot: (i: SnippetInput) => json({
    inputs: [{ type: 'promptString', id: `${NAME}-key`, description: 'CannonCommander MCP key', password: true }],
    servers: { [NAME]: { type: 'http', url: mcpUrl(i), headers: bearerFrom(`\${input:${NAME}-key}`) } },
  }),
  goose: (i: SnippetInput) => [
    'extensions:', `  ${NAME}:`, '    type: streamable_http', `    name: ${NAME}`, '    enabled: true', `    uri: "${mcpUrl(i)}"`, '    headers:',
    `      Authorization: "Bearer \${${KEY_VARIABLE}}"`, '    env_keys:', `      - ${KEY_VARIABLE}`, '    timeout: 300',
  ].join('\n'),
  jan: (i: SnippetInput) => form([['Transport', 'Streamable HTTP'], ['URL', mcpUrl(i)], ['Header', 'Authorization'], ['Value', `Bearer ${i.key}`]]),
  jetbrains: (i: SnippetInput) => servers({ url: mcpUrl(i), headers: bearer(i) }),
  kimi: (i: SnippetInput) => servers({ url: mcpUrl(i), headers: bearer(i) }),
  lmstudio: (i: SnippetInput) => servers({ url: mcpUrl(i), headers: bearer(i) }),
  vibe: (i: SnippetInput) => [
    '[[mcp_servers]]', `name = "${NAME}"`, 'transport = "streamable-http"', `url = "${mcpUrl(i)}"`, `api_key_env = "${KEY_VARIABLE}"`,
    'api_key_header = "Authorization"', 'api_key_format = "Bearer {token}"',
  ].join('\n'),
  msty: (i: SnippetInput) => form([['Name', NAME], ['Connection Type', 'HTTP'], ['Server URL', mcpUrl(i)], ['Authentication', 'Bearer'], ['Token', i.key]]),
  n8n: (i: SnippetInput) => form([['Endpoint URL', mcpUrl(i)], ['Server Transport', 'HTTP Streamable'], ['Authentication', 'Bearer Auth'], ['Credential, Token', i.key]]),
  openwebui: (i: SnippetInput) => form([['Type', 'MCP (Streamable HTTP)'], ['URL', mcpUrl(i)], ['Auth', 'Bearer'], ['Key', i.key]]),
  opencode: (i: SnippetInput) => json({
    $schema: 'https://opencode.ai/config.json',
    mcp: { [NAME]: { type: 'remote', url: mcpUrl(i), enabled: true, headers: bearerFrom(`{env:${KEY_VARIABLE}}`) } },
  }),
  perplexity: (i: SnippetInput) => form([['Server Name', NAME], ['Command', mcpRemote(i)]]),
  qwen: (i: SnippetInput) => servers({ httpUrl: mcpUrl(i), headers: bearerFrom(`\${${KEY_VARIABLE}}`) }),
  roo: (i: SnippetInput) => servers({ type: 'streamable-http', url: mcpUrl(i), headers: bearerFrom(`\${env:${KEY_VARIABLE}}`) }),
  visualstudio: (i: SnippetInput) => json({ servers: { [NAME]: { url: mcpUrl(i), requestInit: { headers: bearer(i) } } } }),
  warp: (i: SnippetInput) => servers({ url: mcpUrl(i), headers: bearer(i) }),
  windsurf: (i: SnippetInput) => servers({ serverUrl: mcpUrl(i), headers: bearerFrom(`\${env:${KEY_VARIABLE}}`) }),
  zed: (i: SnippetInput) => json({ context_servers: { [NAME]: { url: mcpUrl(i), headers: bearer(i) } } }),
  grok: (i: SnippetInput) => form([['MCP server URL', mcpUrl(i)], ['Header', 'Authorization'], ['Value', `Bearer ${i.key}`]]),
  lechat: (i: SnippetInput) => form([['Server URL', mcpUrl(i)], ['Authentication', 'HTTP Bearer Token'], ['Token', i.key]]),
  other: genericSnippet,
} satisfies Record<string, (i: SnippetInput) => string>;

export type SnippetClient = keyof typeof SNIPPETS;
