/// <reference types="vitest/config" />
import { readdirSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { fileURLToPath, URL } from 'node:url';
import vue from '@vitejs/plugin-vue';
import { type Connect, defineConfig, type Plugin } from 'vite';

// Without a server, `--mode fixture` answers /api/v1 from the prototype's test data.
function fixture(): Plugin {
  const use = async (server: { middlewares: Connect.Server }) => {
    const { fixtureMiddleware } = await import('./scripts/fixture.js');
    const { opsFixtureMiddleware } = await import('./scripts/fixture-ops.js');
    const { authFixtureMiddleware } = await import('./scripts/fixture-auth.js');
    const { clockMiddleware } = await import('./scripts/fixture-clock.js');
    server.middlewares.use(clockMiddleware());
    server.middlewares.use(authFixtureMiddleware());
    server.middlewares.use(opsFixtureMiddleware());
    const { settingsFixtureMiddleware } = await import('./scripts/fixture-settings.js');
    server.middlewares.use(settingsFixtureMiddleware());
    server.middlewares.use(fixtureMiddleware());
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

const page = (file: string) => fileURLToPath(new URL(file, import.meta.url));

export default defineConfig(({ mode }) => ({
  plugins: [vue(), sharePage(), shareTexts(), ...(mode === 'fixture' ? [fixture()] : [])],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  // Every source is scanned up front, so a library only a viewer loads later does not make Vite bundle the
  // dependencies again while a page is open.
  optimizeDeps: { entries: ['index.html', 'share.html', 'src/**/*.{ts,vue}', '!src/**/*.test.ts'] },
  server: mode === 'fixture' ? {} : { proxy: { '/api': process.env.CC_BACKEND || 'http://localhost:8080' } },
  build: {
    outDir: 'dist', emptyOutDir: true, chunkSizeWarningLimit: 900,
    rollupOptions: { input: { app: page('./index.html'), share: page('./share.html') } },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
    // The fixture holds dates the prototype wrote out in this zone.
    env: { TZ: 'Europe/Vienna' },
  },
}));
