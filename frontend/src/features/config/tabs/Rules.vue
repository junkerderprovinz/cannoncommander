<script setup lang="ts">
import { pathField } from '@/ui/picker';
import { selectOptions } from '@/ui/kit';
import { Select } from '@glimstone/vue/Select';
import { Button, Card, EmptyState, FieldRow, IconButton, InfoBubble, ListRow, PathField, Pill, type PillTone } from '@glimstone/vue';
import { onMounted, reactive, ref, watch } from 'vue';
import { admin, type Rule, type User, type Verdict } from '@/api/admin';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { c } from '@/features/config/shared';
import { ruleGlyph, ruleName, whoName } from '../roles';
import RuleWin from '../wins/RuleWin.vue';

const rules = ref<Rule[]>([]), users = ref<User[]>([]);
// The probe outlives a visit to another tab, as somebody checking a rule goes back and forth.
const probe = reactive(probeState);
const result = ref<{ word: string; tone: PillTone; why: string }>({ word: '', tone: 'neutral', why: '' });

async function load() {
  [rules.value, users.value] = await Promise.all([admin.rules(), admin.users()]);
  if (!users.value.some((u) => u.name === probe.who)) probe.who = users.value[0]?.name ?? '';
  await check();
}
onMounted(() => { load().catch(() => {}); });

function why(v: Verdict) {
  const r = v.rule == null ? undefined : rules.value[v.rule];
  switch (v.reason) {
    case 'scope': return c('rules.outside', { scope: v.scope ?? '' });
    case 'rule': return r ? c('rules.byRule', { n: v.rule! + 1, who: whoName(r.who, users.value), path: r.path, rule: ruleName(r.rule) }) : c('rules.noneScope');
    case 'role': return c('rules.noneReadonly');
    case 'source': return c('rules.readOnlySource');
    default: return c('rules.noneScope');
  }
}
async function check() {
  const user = users.value.find((u) => u.name === probe.who);
  const path = probe.path.trim();
  if (!user || !path) return;
  const v = await admin.testRule(user.name, path);
  if (v.result === 'missing') {
    result.value = { word: c('rules.missing'), tone: 'warn', why: c('rules.missingWhy') };
    return;
  }
  const word = v.result === 'hidden' ? c('rules.hidden') : v.result === 'read' ? c('rules.readOnly') : c('rules.visible');
  result.value = { word, tone: v.result === 'visible' ? 'ok' : 'state', why: why(v) };
}
watch(() => [probe.who, probe.path], () => { check().catch(() => {}); });

// The list on the page follows the server's, also when it refuses a change.
async function save(next: Rule[]) {
  try {
    await admin.saveRules(next);
  } finally {
    await load().catch(() => {});
  }
}
const quietly = (next: Rule[]) => save(next).catch(() => toast(c('notSaved'), 'warn'));
function move(i: number, d: number) {
  const next = [...rules.value];
  next.splice(i + d, 0, next.splice(i, 1)[0]!);
  quietly(next);
}
function remove(i: number) {
  const before = [...rules.value];
  quietly(rules.value.filter((_, k) => k !== i));
  toast(c('rules.removed'), 'ok', { label: c('undo'), run: () => { quietly(before); } });
}
const edit = (i: number | null) => win({ title: i == null ? c('rules.add') : c('rules.edit'), body: RuleWin, props: { index: i, rules: rules.value, users: users.value, save } });
</script>

<script lang="ts">
const probeState = { who: 'anna', path: '/mnt/user/backups' };
</script>

<template>
  <Card :title="c('rules.title')">
    <div v-if="rules.length" class="a-list">
      <ListRow v-for="(r, i) in rules" :key="i" :name="r.path" mono :data-open="i" @click="edit(i)">
        <template #tile><G :n="ruleGlyph(r.rule)" /></template>
        <template #badges><Pill>{{ whoName(r.who, users) }}</Pill><Pill tone="state">{{ ruleName(r.rule) }}</Pill></template>
        <template #actions>
          <IconButton v-if="i > 0" :glyph="glyphComponent('IconMoveUp')" :tip="c('rules.up')" @click="move(i, -1)" /><span v-else class="glim-icon-btn-gap"></span>
          <IconButton v-if="i < rules.length - 1" :glyph="glyphComponent('IconMoveDown')" :tip="c('rules.down')" @click="move(i, 1)" /><span v-else class="glim-icon-btn-gap"></span>
          <IconButton :glyph="glyphComponent('IconPencil')" :tip="c('edit')" @click="edit(i)" />
          <IconButton :glyph="glyphComponent('IconTrash')" :tip="c('remove')" @click="remove(i)" />
        </template>
      </ListRow>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconShieldOn')" :title="c('rules.none')" />
    <div class="a-foot"><Button data-add :label="c('rules.add')" label-key="IconAdd" @click="edit(null)" /></div>
  </Card>
  <Card :title="c('rules.probe')">
    <FieldRow :label="c('rules.who')"><Select v-model="probe.who" :options="selectOptions(users.map((u) => [u.name, u.full]))" rows="menu" /></FieldRow>
    <FieldRow :label="c('rules.path')"><PathField v-model="probe.path" v-bind="pathField('a-ppath')" /></FieldRow>
    <FieldRow :label="c('rules.result')" :hint="c('rules.resultTip')"><Pill :tone="result.tone">{{ result.word }}</Pill><InfoBubble :tip="result.why" /></FieldRow>
  </Card>
</template>
