<script setup lang="ts">
// The clients to pick from: the ones on this computer, the cloud ones where the server can be reached from outside,
// and the other client last.
import { EmptyState, InfoBubble, ReadmeButton, TextField } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { closeBtn, type WinEntry } from '@/core/windows';
import { c, keysFirst, stepFoot } from '@/features/config/shared';
import ClientMark from './ClientMark.vue';
import { CLOUD_CLIENTS, LOCAL_CLIENTS, type McpClient, OTHER_CLIENT } from './clients';

const props = defineProps<{ win: WinEntry; cloud: boolean; pick: (client: McpClient) => void }>();
const query = ref('');
const nameOf = (x: McpClient) => (x === OTHER_CLIENT ? c('mcp.other') : x.name);
const hit = (x: McpClient) => nameOf(x).toLowerCase().includes(query.value.trim().toLowerCase());
const groups = computed(() => [
  { id: 'local', title: c('mcp.local'), tip: '', clients: LOCAL_CLIENTS.filter(hit) },
  { id: 'cloud', title: c('mcp.cloud'), tip: c('mcp.cloudTip'), clients: props.cloud ? CLOUD_CLIENTS.filter(hit) : [] },
].filter((g) => g.clients.length));
const other = computed(() => hit(OTHER_CLIENT));

function choose(x: McpClient) {
  props.win.close();
  props.pick(x);
}
const parts = (x: McpClient): [{ name: string; sub?: string; onClick: () => void }] => [{ name: nameOf(x), sub: x.kind ? c(`mcp.kind.${x.kind}`) : undefined, onClick: () => choose(x) }];

let stopKeys = () => {};
onMounted(() => {
  stepFoot(props.win, [closeBtn()]);
  setTimeout(() => (props.win.el?.querySelector('#mcp-q') as HTMLInputElement | null)?.focus(), 0);
  // Escape in the search empties it first and closes the window only when it is empty.
  stopKeys = keysFirst((e) => {
    if (e.key !== 'Escape' || !query.value || document.activeElement?.id !== 'mcp-q') return;
    e.preventDefault();
    e.stopPropagation();
    query.value = '';
  });
});
onBeforeUnmount(() => stopKeys());
</script>

<template>
  <div class="a-pickhead"><TextField id="mcp-q" v-model="query" :placeholder="c('mcp.search')" :aria-label="c('mcp.search')" width="medium" /></div>
  <template v-for="g in groups" :key="g.id">
    <h3 class="mcp-group">{{ g.title }}<InfoBubble v-if="g.tip" :tip="g.tip" /></h3>
    <div class="mcp-pick" :data-group="g.id">
      <ReadmeButton v-for="x in g.clients" :key="x.id" brand="house" :class="[x.tile, { 'mcp-lift': x.lift }]" :parts="parts(x)" :data-client="x.id">
        <template #mark><ClientMark :client="x" /></template>
      </ReadmeButton>
    </div>
  </template>
  <div v-if="other" class="mcp-pick" :class="{ 'mcp-alone': groups.length }">
    <ReadmeButton brand="house" :parts="parts(OTHER_CLIENT)" data-client="other"><template #mark><ClientMark :client="OTHER_CLIENT" /></template></ReadmeButton>
  </div>
  <EmptyState v-if="!groups.length && !other" nested :glyph="glyphComponent('IconSearch')" :title="c('mcp.noMatch')" />
</template>
