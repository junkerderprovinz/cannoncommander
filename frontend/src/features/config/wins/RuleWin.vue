<script setup lang="ts">
import { pathField } from '@/ui/picker';
import { segItems, selectOptions } from '@/ui/kit';
import { Select } from '@glimstone/vue/Select';
import { FieldRow, FieldRowGroup, PathField, Pill, Selector, SubCard } from '@glimstone/vue';
import { computed, onMounted, reactive, ref } from 'vue';
import { admin, exists, type Rule, type User } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { attempt, c, fail, stepFoot } from '@/features/config/shared';
import { RULES, ruleName } from '../roles';

const props = defineProps<{ win: WinEntry; index: number | null; rules: Rule[]; users: User[]; save: (rules: Rule[]) => Promise<void> }>();
const isNew = props.index == null;
const d = reactive<Rule>(isNew ? { who: '*', path: '', rule: 'hide' } : { ...props.rules[props.index!]! });
const groups = ref<string[]>([]);
const kind = ref<'user' | 'group' | 'all'>(d.who === '*' ? 'all' : d.who.startsWith('@') ? 'group' : 'user');
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;

// Changing who the rule is for starts from the first person or group.
const kindModel = computed({
  get: () => kind.value,
  set: (k: 'user' | 'group' | 'all') => {
    kind.value = k;
    d.who = k === 'all' ? '*' : k === 'group' ? (groups.value[0] ? '@' + groups.value[0] : '') : props.users[0]?.name ?? '';
  },
});

async function save() {
  if (kind.value === 'group' && !d.who) return fail(c('rules.needGroup'), at('#a-rkind'));
  let path = d.path.trim();
  if (!path) return fail(c('rules.needPath'), at('#a-rpath'));
  if (!path.includes('*')) {
    path = path.replace(/\/+$/, '') || '/';
    if (!path.startsWith('/') || !((await exists(path)) || (await exists(path, 'file')))) return fail(c('rules.noPath'), at('#a-rpath'));
  }
  const rule = { ...d, path };
  const next = isNew ? [...props.rules, rule] : props.rules.map((r, k) => (k === props.index ? rule : r));
  if (!(await attempt(props.save(next), at('#a-rpath')))) return;
  props.win.close();
  toast(isNew ? c('rules.added') : c('saved'));
}

onMounted(async () => {
  stepFoot(props.win, [cancelBtn(), { label: isNew ? c('users.create') : c('save'), g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: () => { save(); return false; } }]);
  groups.value = (await admin.groups().catch(() => [])).map((g) => g.name);
});
</script>

<template>
  <SubCard>
    <FieldRow :label="c('rules.appliesTo')"><Selector id="a-rkind" v-model="kindModel" :items="segItems([['user', c('rules.user')], ['group', c('rules.groupWord')], ['all', c('rules.all')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow v-if="kind === 'user'" :label="c('rules.user')"><Select v-model="d.who" :options="selectOptions(users.map((u) => [u.name, u.full]))" rows="menu" /></FieldRow>
    <FieldRow v-else-if="kind === 'group'" :label="c('rules.groupWord')">
      <Select v-if="groups.length" v-model="d.who" :options="selectOptions(groups.map((g) => ['@' + g, g]))" rows="menu" />
      <Pill v-else tone="state">{{ c('users.noGroups') }}</Pill>
    </FieldRow>
  </SubCard>
  <SubCard>
    <FieldRowGroup>
      <FieldRow :label="c('rules.pathOrPattern')" :hint="c('rules.pathTip')"><PathField v-model="d.path" :placeholder="c('rules.pathPh')" v-bind="pathField('a-rpath')" /></FieldRow>
      <FieldRow :label="c('rules.rule')" :hint="c('rules.ruleTip')"><Selector v-model="d.rule" :items="segItems(RULES.map(([r, g]) => [r, ruleName(r), g]))" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    </FieldRowGroup>
  </SubCard>
</template>
