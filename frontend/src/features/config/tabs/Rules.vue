<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue';
import { admin, exists, type Rule, type User, type Verdict } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import ARow from '../kit/ARow.vue';
import BareBtn from '../kit/BareBtn.vue';
import Btn from '../kit/Btn.vue';
import Card from '../kit/Card.vue';
import Dropdown from '../kit/Dropdown.vue';
import Frow from '../kit/Frow.vue';
import Info from '../kit/Info.vue';
import { c } from '../kit/kit';
import PathField from '../kit/PathField.vue';
import { ruleGlyph, ruleName, whoName } from '../roles';
import RuleWin from '../wins/RuleWin.vue';

const rules = ref<Rule[]>([]), users = ref<User[]>([]);
// The probe outlives a visit to another tab, as somebody checking a rule goes back and forth.
const probe = reactive(probeState);
const result = ref<{ word: string; tone: string; why: string }>({ word: '', tone: '', why: '' });

async function load() {
  [rules.value, users.value] = await Promise.all([admin.rules(), admin.users()]);
  if (!users.value.some((u) => u.name === probe.who)) probe.who = users.value[0]?.name ?? '';
  await check();
}
onMounted(() => { load().catch(() => {}); });

function why(v: Verdict, user: User) {
  if (v.scope) return c('rules.outside', { scope: v.scope });
  if (v.rule == null) return user.role === 'readonly' ? c('rules.noneReadonly') : c('rules.noneScope');
  const r = rules.value[v.rule]!;
  return c('rules.byRule', { n: v.rule + 1, who: whoName(r.who, users.value), path: r.path, rule: ruleName(r.rule) });
}
async function check() {
  const user = users.value.find((u) => u.name === probe.who);
  if (!user) return;
  const path = probe.path.trim();
  if (!(await exists(path)) && !(await exists(path, 'file'))) {
    result.value = { word: c('rules.missing'), tone: 'warn', why: c('rules.missingWhy') };
    return;
  }
  const v = await admin.testRule(user.name, path);
  const word = v.result === 'hidden' ? c('rules.hidden') : v.result === 'read' ? c('rules.readOnly') : c('rules.visible');
  result.value = { word, tone: v.result === 'visible' ? 'ok' : 'state', why: why(v, user) };
}
watch(() => [probe.who, probe.path], () => { check().catch(() => {}); });

async function save(next: Rule[]) {
  await admin.saveRules(next);
  await load();
}
function move(i: number, d: number) {
  const next = [...rules.value];
  next.splice(i + d, 0, next.splice(i, 1)[0]!);
  save(next);
}
function remove(i: number) {
  const before = [...rules.value];
  save(rules.value.filter((_, k) => k !== i));
  toast(c('rules.removed'), 'ok', { label: c('undo'), run: () => { save(before); } });
}
const edit = (i: number | null) => win({ title: i == null ? c('rules.add') : c('rules.edit'), body: RuleWin, props: { index: i, rules: rules.value, users: users.value, save } });
</script>

<script lang="ts">
const probeState = { who: 'anna', path: '/mnt/user/backups' };
</script>

<template>
  <Card :title="c('rules.title')">
    <div v-if="rules.length" class="a-list">
      <ARow v-for="(r, i) in rules" :key="i" :name="r.path" mono data-open @click="edit(i)">
        <template #tile><G :n="ruleGlyph(r.rule)" /></template>
        <template #badges><span class="pill">{{ whoName(r.who, users) }}</span><span class="pill state">{{ ruleName(r.rule) }}</span></template>
        <template #acts>
          <BareBtn :g="i > 0 ? 'IconMoveUp' : undefined" :tip="c('rules.up')" @click="move(i, -1)" />
          <BareBtn :g="i < rules.length - 1 ? 'IconMoveDown' : undefined" :tip="c('rules.down')" @click="move(i, 1)" />
          <BareBtn g="IconPencil" :tip="c('edit')" @click="edit(i)" />
          <BareBtn g="IconTrash" :tip="c('remove')" @click="remove(i)" />
        </template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconShieldOn" /><div>{{ c('rules.none') }}</div></div>
    <div class="a-foot"><Btn :label="c('rules.add')" g="IconAdd" @click="edit(null)" /></div>
  </Card>
  <Card :title="c('rules.probe')">
    <Frow :label="c('rules.who')"><Dropdown v-model="probe.who" :options="users.map((u) => [u.name, u.full])" /></Frow>
    <Frow :label="c('rules.path')"><PathField v-model="probe.path" /></Frow>
    <Frow :label="c('rules.result')" :tip="c('rules.resultTip')"><span id="a-pres" class="a-ctl"><span class="pill" :class="result.tone">{{ result.word }}</span><Info :tip="result.why" /></span></Frow>
  </Card>
</template>
