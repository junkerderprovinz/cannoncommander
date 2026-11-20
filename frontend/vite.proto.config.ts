// Prototype build of the job window variants: npm run proto:dialogs writes a static page to dist-proto.
import vue from '@vitejs/plugin-vue';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

export default defineConfig({
  root: fileURLToPath(new URL('./prototype-job-dialogs', import.meta.url)),
  base: './',
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  build: { outDir: fileURLToPath(new URL('./dist-proto', import.meta.url)), emptyOutDir: true },
});
