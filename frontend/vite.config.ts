/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url';
import vue from '@vitejs/plugin-vue';
import { type Connect, defineConfig, type Plugin } from 'vite';

// Without a server, `--mode fixture` answers /api/v1 from the prototype's test data.
function fixture(): Plugin {
  const use = async (server: { middlewares: Connect.Server }) => {
    const { fixtureMiddleware } = await import('./scripts/fixture.js');
    const { opsFixtureMiddleware } = await import('./scripts/fixture-ops.js');
    server.middlewares.use(opsFixtureMiddleware());
    const { settingsFixtureMiddleware } = await import('./scripts/fixture-settings.js');
    server.middlewares.use(settingsFixtureMiddleware());
    server.middlewares.use(fixtureMiddleware());
  };
  return { name: 'cc-fixture', configureServer: use, configurePreviewServer: use };
}

export default defineConfig(({ mode }) => ({
  plugins: [vue(), ...(mode === 'fixture' ? [fixture()] : [])],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: mode === 'fixture' ? {} : { proxy: { '/api': process.env.CC_BACKEND || 'http://localhost:8080' } },
  build: { outDir: 'dist', emptyOutDir: true, chunkSizeWarningLimit: 900 },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
  },
}));
