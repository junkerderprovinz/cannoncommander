// The MCP clients the card offers a tile for, and what each one's setup window says: where its configuration lives
// and how it comes by the key without holding it. The facts come from each client's own documentation.
import { SNIPPETS, type SnippetClient, type SnippetInput } from './snippets';

// The second line of a client's tile, a key under mcp.kind.
export type ClientKind = 'terminal' | 'editor' | 'desktop' | 'vscode' | 'localModels' | 'automation' | 'webChat';

// Where a configuration file lives. unix stands for macOS and Linux alike.
export interface ConfigPaths { windows?: string; macos?: string; linux?: string; unix?: string }

export type SetupWay =
  | { kind: 'command' }
  // ui opens the file in the client; paste takes the same text pasted in.
  | { kind: 'file'; paths: ConfigPaths; ui?: string; paste?: string }
  | { kind: 'form'; ui: string }
  | { kind: 'other' };

// How the client comes by the key: mcp-remote reading a file, an environment variable (unsure where the documentation
// shows the variable in the other fields but not in a header), a masked prompt, a dotenv line, the configuration file
// itself, the client's own settings, an encrypted credential store, or the vendor's servers.
export type KeyWay =
  | { kind: 'keyFile' } | { kind: 'env'; unsure?: boolean } | { kind: 'prompt' } | { kind: 'dotenv'; file: string }
  | { kind: 'inFile' } | { kind: 'inApp' } | { kind: 'credential' } | { kind: 'vendor' } | { kind: 'other' };

export interface McpClient {
  // Stored with a key made for this client, so it never changes. ChatGPT and Claude carry the ids the server gives
  // the OAuth clients it recognises.
  id: string;
  name: string;
  kind?: ClientKind;
  group: 'local' | 'cloud';
  // The entry in CLIENT_MARKS; the other client draws a glyph.
  mark?: string;
  // The tile colour its button lights up in, from config.css.
  tile: string;
  // Brightens the resting mark on the dark theme, where its colour would read as a hole in the button.
  lift?: boolean;
  setup: SetupWay;
  key: KeyWay;
  // A cloud service that signs in through OAuth instead of taking a key, with its steps under mcp.setup.
  oauth?: string;
  // The window opens with the sentence under mcp.note.
  note?: string;
  // The client runs mcp-remote through npx.
  node?: boolean;
}

function vsCodeStorage(rest: string): ConfigPaths {
  return {
    windows: `%APPDATA%\\Code\\User\\${rest.replaceAll('/', '\\')}`,
    macos: `~/Library/Application Support/Code/User/${rest}`,
    linux: `~/.config/Code/User/${rest}`,
  };
}
const home = (rest: string): ConfigPaths => ({ windows: `%USERPROFILE%\\${rest.replaceAll('/', '\\')}`, unix: `~/${rest}` });

export const LOCAL_CLIENTS: McpClient[] = [
  {
    id: 'anythingllm', name: 'AnythingLLM', kind: 'localModels', group: 'local', mark: 'anythingllm', tile: 'glim-tile-anythingllm',
    setup: {
      kind: 'file', ui: 'Agent Skills, MCP Servers',
      paths: {
        windows: '%APPDATA%\\anythingllm-desktop\\storage\\plugins\\anythingllm_mcp_servers.json',
        macos: '~/Library/Application Support/anythingllm-desktop/storage/plugins/anythingllm_mcp_servers.json',
        linux: '~/.config/anythingllm-desktop/storage/plugins/anythingllm_mcp_servers.json',
      },
    },
    key: { kind: 'inFile' },
  },
  {
    id: 'antigravity', name: 'Antigravity', kind: 'editor', group: 'local', mark: 'antigravity', tile: 'glim-tile-antigravity',
    setup: { kind: 'file', paths: home('.gemini/config/mcp_config.json'), ui: 'Manage MCP Servers, View raw config' }, key: { kind: 'env' },
  },
  { id: 'claude-code', name: 'Claude Code', kind: 'terminal', group: 'local', mark: 'claudecode', tile: 'glim-tile-claude', setup: { kind: 'command' }, key: { kind: 'keyFile' }, node: true },
  {
    id: 'claude-desktop', name: 'Claude Desktop', kind: 'desktop', group: 'local', mark: 'claude', tile: 'glim-tile-claude',
    setup: {
      kind: 'file', ui: 'Settings, Developer, Edit Config',
      paths: { windows: '%APPDATA%\\Claude\\claude_desktop_config.json', macos: '~/Library/Application Support/Claude/claude_desktop_config.json' },
    },
    key: { kind: 'keyFile' }, node: true,
  },
  {
    id: 'cline', name: 'Cline', kind: 'vscode', group: 'local', mark: 'cline', tile: 'glim-tile-cline',
    setup: { kind: 'file', paths: vsCodeStorage('globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json'), ui: 'MCP Servers, Configure MCP Servers' },
    key: { kind: 'inFile' },
  },
  { id: 'codex', name: 'Codex CLI', kind: 'terminal', group: 'local', mark: 'openai', tile: 'glim-tile-openai', setup: { kind: 'file', paths: home('.codex/config.toml') }, key: { kind: 'env' } },
  {
    id: 'continue', name: 'Continue', kind: 'vscode', group: 'local', mark: 'continue', tile: 'glim-tile-continue',
    setup: { kind: 'file', paths: home('.continue/config.yaml') }, key: { kind: 'dotenv', file: '~/.continue/.env' },
  },
  { id: 'copilot-cli', name: 'Copilot CLI', kind: 'terminal', group: 'local', mark: 'copilot', tile: 'glim-tile-copilot', setup: { kind: 'file', paths: home('.copilot/mcp-config.json') }, key: { kind: 'inFile' } },
  {
    id: 'cursor', name: 'Cursor', kind: 'editor', group: 'local', mark: 'cursor', tile: 'glim-tile-cursor',
    setup: { kind: 'file', paths: home('.cursor/mcp.json'), ui: 'Settings, MCP, Add new global MCP server' }, key: { kind: 'env' },
  },
  { id: 'gemini', name: 'Gemini CLI', kind: 'terminal', group: 'local', mark: 'gemini', tile: 'glim-tile-gemini', setup: { kind: 'file', paths: home('.gemini/settings.json') }, key: { kind: 'env', unsure: true } },
  {
    id: 'copilot', name: 'GitHub Copilot', kind: 'vscode', group: 'local', mark: 'vscode', tile: 'glim-tile-vscode',
    setup: { kind: 'file', paths: vsCodeStorage('mcp.json'), ui: 'MCP: Open User Configuration' }, key: { kind: 'prompt' },
  },
  {
    id: 'goose', name: 'Goose', kind: 'desktop', group: 'local', mark: 'goose', tile: 'glim-tile-goose',
    setup: { kind: 'file', paths: { windows: '%APPDATA%\\Block\\goose\\config\\config.yaml', unix: '~/.config/goose/config.yaml' } }, key: { kind: 'env', unsure: true },
  },
  { id: 'jan', name: 'Jan', kind: 'localModels', group: 'local', mark: 'jan', tile: 'glim-tile-jan', setup: { kind: 'form', ui: 'Settings, MCP Servers, Add MCP Server' }, key: { kind: 'inApp' } },
  {
    id: 'jetbrains', name: 'JetBrains', kind: 'editor', group: 'local', mark: 'jetbrains', tile: 'glim-tile-jetbrains',
    setup: { kind: 'file', paths: home('.junie/mcp/mcp.json'), ui: 'Settings, Tools, Junie, MCP Settings', paste: 'Settings, Tools, AI Assistant, Model Context Protocol (MCP), Add' },
    key: { kind: 'inFile' },
  },
  { id: 'kimi', name: 'Kimi Code', kind: 'terminal', group: 'local', mark: 'kimi', tile: 'glim-tile-kimi', setup: { kind: 'file', paths: home('.kimi/mcp.json') }, key: { kind: 'inFile' } },
  {
    id: 'lmstudio', name: 'LM Studio', kind: 'localModels', group: 'local', mark: 'lmstudio', tile: 'glim-tile-lmstudio',
    setup: { kind: 'file', paths: home('.lmstudio/mcp.json'), ui: 'Program, Install, Edit mcp.json' }, key: { kind: 'inFile' },
  },
  { id: 'vibe', name: 'Mistral Vibe', kind: 'terminal', group: 'local', mark: 'mistral', tile: 'glim-tile-mistral', setup: { kind: 'file', paths: home('.vibe/config.toml') }, key: { kind: 'env' } },
  { id: 'msty', name: 'Msty', kind: 'localModels', group: 'local', mark: 'msty', tile: 'glim-tile-msty', lift: true, setup: { kind: 'form', ui: 'Toolbox, Tools, Add New Tool' }, key: { kind: 'inApp' } },
  { id: 'n8n', name: 'n8n', kind: 'automation', group: 'local', mark: 'n8n', tile: 'glim-tile-n8n', setup: { kind: 'form', ui: 'MCP Client Tool' }, key: { kind: 'credential' } },
  {
    id: 'openwebui', name: 'Open WebUI', kind: 'localModels', group: 'local', mark: 'openwebui', tile: 'glim-tile-openwebui',
    setup: { kind: 'form', ui: 'Admin Panel, Settings, External Tools, Add Connection' }, key: { kind: 'inApp' },
  },
  { id: 'opencode', name: 'opencode', kind: 'terminal', group: 'local', mark: 'opencode', tile: 'glim-tile-opencode', setup: { kind: 'file', paths: home('.config/opencode/opencode.json') }, key: { kind: 'env' } },
  {
    id: 'perplexity', name: 'Perplexity', kind: 'desktop', group: 'local', mark: 'perplexity', tile: 'glim-tile-perplexity',
    setup: { kind: 'form', ui: 'Settings, Connectors, Add Connector, MCP Connector' }, key: { kind: 'keyFile' }, note: 'perplexity', node: true,
  },
  { id: 'qwen', name: 'Qwen Code', kind: 'terminal', group: 'local', mark: 'qwen', tile: 'glim-tile-qwen', setup: { kind: 'file', paths: home('.qwen/settings.json') }, key: { kind: 'env', unsure: true } },
  {
    id: 'roo', name: 'Roo Code', kind: 'vscode', group: 'local', mark: 'roocode', tile: 'glim-tile-roocode',
    setup: { kind: 'file', paths: vsCodeStorage('globalStorage/rooveterinaryinc.roo-cline/settings/mcp_settings.json'), ui: 'MCP, Edit Global MCP' }, key: { kind: 'env' },
  },
  {
    id: 'visualstudio', name: 'Visual Studio', kind: 'editor', group: 'local', mark: 'visualstudio', tile: 'glim-tile-visualstudio', lift: true,
    setup: { kind: 'file', paths: { windows: '%USERPROFILE%\\.mcp.json' } }, key: { kind: 'inFile' },
  },
  {
    id: 'warp', name: 'Warp', kind: 'terminal', group: 'local', mark: 'warp', tile: 'glim-tile-warp',
    setup: { kind: 'file', paths: { unix: '~/.warp/.mcp.json' }, paste: 'Settings, Agents, Warp Agent, Manage MCP servers, + Add' }, key: { kind: 'inFile' },
  },
  {
    id: 'windsurf', name: 'Windsurf', kind: 'editor', group: 'local', mark: 'windsurf', tile: 'glim-tile-windsurf',
    setup: { kind: 'file', paths: { windows: '%APPDATA%\\devin\\mcp_config.json', unix: '~/.config/devin/mcp_config.json' } }, key: { kind: 'env' },
  },
  {
    id: 'zed', name: 'Zed', kind: 'editor', group: 'local', mark: 'zed', tile: 'glim-tile-zed',
    setup: { kind: 'file', paths: { windows: '%APPDATA%\\Zed\\settings.json', unix: '~/.config/zed/settings.json' } }, key: { kind: 'inFile' },
  },
];

export const CLOUD_CLIENTS: McpClient[] = [
  { id: 'chatgpt', name: 'ChatGPT', kind: 'webChat', group: 'cloud', mark: 'chatgpt', tile: 'glim-tile-chatgpt', setup: { kind: 'other' }, key: { kind: 'vendor' }, oauth: 'chatgpt' },
  { id: 'claude', name: 'Claude', kind: 'webChat', group: 'cloud', mark: 'claude', tile: 'glim-tile-claude', setup: { kind: 'other' }, key: { kind: 'vendor' }, oauth: 'claude' },
  {
    id: 'grok', name: 'Grok', kind: 'webChat', group: 'cloud', mark: 'grok', tile: 'glim-tile-grok',
    setup: { kind: 'form', ui: 'grok.com, Connectors, New Connector, Custom' }, key: { kind: 'vendor' }, note: 'grok',
  },
  { id: 'lechat', name: 'Le Chat', kind: 'webChat', group: 'cloud', mark: 'mistral', tile: 'glim-tile-mistral', setup: { kind: 'form', ui: 'Connectors, Add a Custom Connector' }, key: { kind: 'vendor' } },
];

export const OTHER_CLIENT: McpClient = { id: 'other', name: '', group: 'local', tile: 'glim-tile-house', setup: { kind: 'other' }, key: { kind: 'other' } };

const BY_ID = new Map([...LOCAL_CLIENTS, ...CLOUD_CLIENTS, OTHER_CLIENT].map((c) => [c.id, c]));

// The client a key was made for, or undefined for one made through the other client or for a client since dropped.
export const clientById = (id: string | undefined) => (id && id !== OTHER_CLIENT.id ? BY_ID.get(id) : undefined);

// The configuration to copy for a client, or undefined for one that signs in through OAuth instead.
export function snippetFor(client: McpClient, input: SnippetInput): string | undefined {
  return client.id in SNIPPETS && !client.oauth ? SNIPPETS[client.id as SnippetClient](input) : undefined;
}
