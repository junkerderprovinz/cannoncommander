/// <reference types="vitest/config" />
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

const page = (file: string) => fileURLToPath(new URL(file, import.meta.url));

export default defineConfig(({ mode }) => ({
  plugins: [vue(), sharePage(), ...(mode === 'fixture' ? [fixture()] : [])],
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
