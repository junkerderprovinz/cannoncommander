<script setup lang="ts">
// Im Editor umbenennen: every name on a line of its own, edited as text, and each changed line renames its file.
import { Button, Pill, TextField, Toggle } from '@glimstone/vue';
import { computed, onMounted, ref, watchEffect } from 'vue';
import type { RenameError } from '@/api/tools';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { plural } from '@/ui/kit';

export interface EditRow { old: string; new: string; error?: RenameError }
export interface EditState { dir: string; nodes: { name: string; at: string }[]; text: string; rows: EditRow[]; lines: number }
const props = defineProps<{ st: EditState }>();
const listing = useListing();

const find = ref(''), repl = ref(''), regex = ref(false);
const area = ref<HTMLTextAreaElement | null>(null), gutter = ref<HTMLElement | null>(null);

// One empty line at the end is what an editor leaves behind and does not count as a name.
const names = computed(() => props.st.text.replace(/\n$/, '').split('\n'));
watchEffect(() => {
  const st = props.st, moving = new Set(st.nodes.map((n) => n.at + '/' + n.name));
  const taken = (at: string) => new Set((listing.lists.get(at)?.items ?? []).filter((k) => !moving.has(at + '/' + k.name)).map((k) => k.name.toLowerCase()));
  const count = new Map<string, number>();
  names.value.forEach((n) => count.set(n.trim().toLowerCase(), (count.get(n.trim().toLowerCase()) || 0) + 1));
  st.lines = names.value.length;
  st.rows = st.nodes.map((node, i) => {
    const out = (names.value[i] ?? '').trim();
    const error: RenameError | undefined = !out ? 'empty' : /[/\0]/.test(out) ? 'invalid' : out !== node.name && taken(node.at).has(out.toLowerCase()) ? 'exists'
      : count.get(out.toLowerCase())! > 1 ? 'duplicate' : undefined;
    return { old: node.name, new: out, ...(error ? { error } : {}) };
  });
});

const pattern = computed(() => {
  if (!find.value) return null;
  try {
    return regex.value ? new RegExp(find.value, 'g') : new RegExp(find.value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
  } catch {
    return undefined;
  }
});
const hits = computed(() => (pattern.value === undefined ? t('rename.exprBad') : pattern.value ? plural((props.st.text.match(pattern.value) || []).length, 'rename.hits') : ''));
const changed = computed(() => props.st.rows.filter((r) => r.new !== r.old).length);
const total = computed(() => props.st.nodes.length);
const warning = computed(() => (props.st.lines > total.value
  ? t('rename.tooMany', { lines: props.st.lines, names: total.value, drop: plural(props.st.lines - total.value, 'rename.lines') })
  : t('rename.tooFew', { lines: props.st.lines, names: total.value })));

function replaceAll() {
  if (!pattern.value) return;
  props.st.text = props.st.text.replace(pattern.value, repl.value);
  area.value?.focus();
}
const follow = () => { if (gutter.value && area.value) gutter.value.scrollTop = area.value.scrollTop; };
// The window puts the focus on its first field once it is drawn; the names come first here.
onMounted(() => setTimeout(() => area.value?.focus(), 20));
</script>

<template>
  <div class="well glim-well re-find">
    <TextField id="re-q" v-model="find" :placeholder="t('rename.find')" :aria-label="t('rename.find')" spellcheck="false" autocomplete="off" width="grow" mono />
    <TextField id="re-r" v-model="repl" :placeholder="t('rename.replace')" :aria-label="t('rename.replace')" spellcheck="false" autocomplete="off" width="grow" mono />
    <span class="re-sw">{{ t('rename.expr') }} <Toggle id="re-regex" v-model="regex" :data-tip="t('rename.exprTip')" /></span>
    <Pill id="re-n" class="glim-num">{{ hits }}</Pill>
    <Button id="re-all" :label="t('rename.replaceAll')" label-key="IconSync" :disabled="!pattern" @click="replaceAll" />
  </div>
  <div class="re-main">
    <div class="well glim-well re-edit">
      <div ref="gutter" class="re-gutter mono" aria-hidden="true"><span v-for="(_, i) in names" :key="i" :class="{ extra: i >= total }">{{ i + 1 }}</span></div>
      <textarea ref="area" v-model="st.text" class="re-ta mono" spellcheck="false" autocapitalize="off" wrap="off" :aria-label="t('rename.names')" @scroll="follow"></textarea>
    </div>
    <div class="well glim-well bk-fill"><div class="bk-tools"><span id="re-sum" class="eyebrow">{{ t('rename.count', { objects: plural(total, 'ui.objects'), changes: plural(changed, 'rename.changes') }) }}</span></div>
      <div v-if="st.lines !== total" id="re-warn" class="re-warn">{{ warning }}</div>
      <div id="re-list" class="bk-list">
        <div v-for="r in st.rows" :key="r.old" class="bk-mrow" :class="{ same: r.new === r.old }">
          <span class="bk-name mono">{{ r.old }}</span><span class="bk-arrow sm"><G n="IconForward" /></span>
          <span class="bk-name mono" :class="{ new: !r.error }">{{ r.new || ' ' }}</span>
          <Pill v-if="r.error" tone="fail">{{ t(`rename.errors.${r.error}`) }}</Pill><span v-else></span>
        </div>
      </div>
    </div>
  </div>
</template>
