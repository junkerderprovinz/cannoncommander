<script setup lang="ts">
// A document in the office suite the server works with. OnlyOffice and Euro-Office share one editor API; Collabora
// takes a form post with the WOPI access token into its frame.
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { type OfficeConfig, officeConfig } from '../content';

const props = defineProps<{ path: string; edit?: boolean }>();
const emit = defineEmits<{ (e: 'failed'): void; (e: 'ready', kind: OfficeConfig['kind']): void }>();

const id = 'cc-office-' + Math.random().toString(36).slice(2);
const form = ref<HTMLFormElement | null>(null);
const cfg = ref<OfficeConfig | null>(null);
let editor: { destroyEditor?: () => void } | null = null;

type DocsApi = { DocEditor: new (id: string, config: Record<string, unknown>) => { destroyEditor?: () => void } };
const scripts = new Map<string, Promise<void>>();
function load(src: string) {
  if (!scripts.has(src)) {
    scripts.set(src, new Promise((done, fail) => {
      const s = Object.assign(document.createElement('script'), { src, async: true, onload: () => done(), onerror: () => { scripts.delete(src); fail(new Error(src)); } });
      document.head.appendChild(s);
    }));
  }
  return scripts.get(src)!;
}

onMounted(async () => {
  try {
    const c = await officeConfig(props.path);
    cfg.value = c;
    if (c.kind === 'collabora') {
      setTimeout(() => form.value?.submit(), 0);
    } else {
      await load(c.url.replace(/\/$/, '') + '/web-apps/apps/api/documents/api.js');
      const api = (window as unknown as { DocsAPI?: DocsApi }).DocsAPI;
      if (!api) throw new Error('no DocsAPI');
      const config = { ...c.config, width: '100%', height: '100%', ...(props.edit ? {} : { type: 'embedded', editorConfig: { ...(c.config?.editorConfig as object), mode: 'view' } }) };
      editor = new api.DocEditor(id, config);
    }
    emit('ready', c.kind);
  } catch {
    emit('failed');
  }
});
onBeforeUnmount(() => editor?.destroyEditor?.());
</script>

<template>
  <div class="cv-office">
    <template v-if="cfg?.kind === 'collabora'">
      <form ref="form" :action="cfg.url" method="post" :target="id">
        <input type="hidden" name="access_token" :value="cfg.token">
        <input type="hidden" name="access_token_ttl" :value="cfg.ttl ?? 0">
      </form>
      <iframe :name="id" :title="path" allow="clipboard-read; clipboard-write; fullscreen"></iframe>
    </template>
    <div v-else :id="id"></div>
  </div>
</template>
