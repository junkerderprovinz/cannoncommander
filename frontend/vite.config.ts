/// <reference types="vitest/config" />
import { readdirSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { fileURLToPath, URL } from 'node:url';
import vue from '@vitejs/plugin-vue';
import { type Connect, defineConfig, type HttpServer, type Plugin } from 'vite';
import { LANGUAGES } from './src/i18n/languages';

// Without a server, `--mode fixture` answers /api/v1 from the prototype's test data.
function fixture(): Plugin {
  const use = async (server: { middlewares: Connect.Server; httpServer: HttpServer | null }) => {
    const { fixtureMiddleware } = await import('./scripts/fixture.js');
    const { opsFixtureMiddleware } = await import('./scripts/fixture-ops.js');
    const { authFixtureMiddleware } = await import('./scripts/fixture-auth.js');
    const { clockMiddleware } = await import('./scripts/fixture-clock.js');
    server.middlewares.use(clockMiddleware());
    server.middlewares.use(authFixtureMiddleware());
    server.middlewares.use(opsFixtureMiddleware());
    const { settingsFixtureMiddleware } = await import('./scripts/fixture-settings.js');
    const { terminalUpgrade } = await import('./scripts/fixture-terminal.js');
    const settings = settingsFixtureMiddleware();
    server.middlewares.use(settings);
    server.middlewares.use(fixtureMiddleware());
    server.httpServer?.on('upgrade', terminalUpgrade(settings.terminalOn));
  };
  return { name: 'cc-fixture', configureServer: use, configurePreviewServer: use };
}

// The server answers /s/{slug} with the page for visitors, and the dev server does the same.
function sharePage(): Plugin {
  const use = (server: { middlewares: Connect.Server }) => {
    server.middlewares.use((req, _res, next) => {
      if (req.url && /^\/s\/[^/?]+\/?(\?|$)/.test(req.url)) req.url = '/share.html';
      next();
    });
  };
  return { name: 'cc-share-page', configureServer: use, configurePreviewServer: use };
}

// The share page asks for a language with ?share and gets the global texts, the controls' and its own, not the app's.
function shareTexts(): Plugin {
  const appOnly = new Set([...readdirSync(fileURLToPath(new URL('./src/features', import.meta.url))), 'signin']);
  return {
    name: 'cc-share-texts',
    enforce: 'pre',
    async load(id) {
      const [file, query] = id.split('?');
      if (query !== 'share' || !file!.endsWith('.json')) return;
      const all: Record<string, unknown> = JSON.parse(await readFile(file!, 'utf8'));
      return `export default ${JSON.stringify(Object.fromEntries(Object.entries(all).filter(([k]) => !appOnly.has(k))))}`;
    },
  };
}

// Android installs the app from a manifest it can fetch again, so every language gets one beside the German file,
// with the description in that language where it is translated.
type Texts = { manifest?: string; install?: { manifest?: string } };
function manifests(): Plugin {
  const src = (file: string) => fileURLToPath(new URL(file, import.meta.url));
  const made = async () => {
    const base = JSON.parse(await readFile(src('./public/manifest.webmanifest'), 'utf8'));
    const said = async (file: string, pick: (table: Texts) => string | undefined) => {
      try { return pick(JSON.parse(await readFile(src(file), 'utf8'))); } catch { return undefined; }
    };
    const english = await said('./src/features/install/en.json', (t) => t.manifest);
    const out: Record<string, string> = {};
    for (const l of LANGUAGES) {
      const own = l.code === 'de' ? base.description : l.code === 'en' ? english : await said(`./src/i18n/locales/${l.code}.json`, (t) => t.install?.manifest);
      out[l.code] = JSON.stringify({ ...base, lang: l.code, dir: l.rtl ? 'rtl' : 'ltr', description: own || english }, null, 2);
    }
    return out;
  };
  return {
    name: 'cc-manifests',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const code = req.url?.match(/^\/manifest\/([\w-]+)\.webmanifest$/)?.[1];
        const text = code && (await made())[code];
        if (!text) return next();
        res.setHeader('Content-Type', 'application/manifest+json');
        res.end(text);
      });
    },
    async generateBundle() {
      for (const [code, text] of Object.entries(await made())) this.emitFile({ type: 'asset', fileName: `manifest/${code}.webmanifest`, source: text });
    },
  };
}

const page = (file: string) => fileURLToPath(new URL(file, import.meta.url));

export default defineConfig(({ mode }) => ({
  plugins: [vue(), sharePage(), shareTexts(), manifests(), ...(mode === 'fixture' ? [fixture()] : [])],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  // Every source is scanned up front, so a library only a viewer loads later does not make Vite bundle the
  // dependencies again while a page is open.
  optimizeDeps: { entries: ['index.html', 'share.html', 'src/**/*.{ts,vue}', '!src/**/*.test.ts'] },
  server: mode === 'fixture' ? {} : { proxy: { '/api': { target: process.env.CC_BACKEND || 'http://localhost:8080', ws: true } } },
  build: {
    outDir: 'dist', emptyOutDir: true, chunkSizeWarningLimit: 900,
    rollupOptions: { input: { app: page('./index.html'), share: page('./share.html') } },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
    setupFiles: ['src/testing/jsdom.ts'],
    // The fixture holds dates the prototype wrote out in this zone.
    env: { TZ: 'Europe/Vienna' },
  },
}));
