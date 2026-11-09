<script setup lang="ts">
// What one key did, newest first: its tool calls and the requests the endpoint turned away. It loads when it opens.
import { EmptyState, SubCard } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { mcp, type McpEvent, type McpKey } from '@/api/mcp';
import { fmtTime } from '@/core/format';
import { glyphComponent } from '@/core/glyphs';
import { closeBtn, type WinEntry } from '@/core/windows';
import { c, cn, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry; k: McpKey }>();
const events = ref<McpEvent[] | null>(null), failed = ref(false);

// A code the server adds later shows as it came.
const KNOWN = ['ok', 'not_permitted', 'read_only_key', 'rate_limited', 'batch_refused', 'hourly_limit', 'too_many', 'too_big', 'invalid_arguments', 'not_found', 'timeout', 'failed'];
const outcome = (e: McpEvent) => (KNOWN.includes(e.outcome) ? c(`mcp.outcome.${e.outcome}`) : c('mcp.outcome.other', { code: e.outcome }));

onMounted(() => {
  stepFoot(props.win, [closeBtn()]);
  mcp.activity(props.k.id).then((e) => { events.value = e; }, () => { failed.value = true; });
});
</script>

<template>
  <SubCard :title="cn(k.callsToday, 'mcp.callsToday')" :hint="c('mcp.logKept')">
    <template v-if="events?.length">
      <div class="mcp-log">
        <div v-for="(e, i) in events" :key="i" :data-outcome="e.outcome">
          <span class="mcp-muted glim-num">{{ fmtTime(e.at) }}</span>
          <span v-if="e.tool" class="mono">{{ e.tool }}</span><span v-else>{{ c('mcp.request') }}</span>
          <span :class="{ warn: e.outcome !== 'ok' }">{{ outcome(e) }}</span>
        </div>
      </div>
    </template>
    <EmptyState v-else-if="failed" nested :glyph="glyphComponent('IconWarning')" :title="c('notSaved')" />
    <EmptyState v-else-if="events" nested :glyph="glyphComponent('IconLog')" :title="c('mcp.logEmpty')" />
  </SubCard>
</template>
