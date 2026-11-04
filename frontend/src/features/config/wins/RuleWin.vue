<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { admin, exists, type Rule, type User } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Dropdown from '../kit/Dropdown.vue';
import Frow from '../kit/Frow.vue';
import { attempt, c, fail, setFoot } from '../kit/kit';
import PathField from '../kit/PathField.vue';
import Seg from '../kit/Seg.vue';
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
  setFoot(props.win, [cancelBtn(), { label: isNew ? c('users.create') : c('save'), g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: () => { save(); return false; } }]);
  groups.value = (await admin.groups().catch(() => [])).map((g) => g.name);
});
</script>

<template>
  <div class="well">
    <Frow :label="c('rules.appliesTo')"><Seg id="a-rkind" v-model="kindModel" :options="[['user', c('rules.user')], ['group', c('rules.groupWord')], ['all', c('rules.all')]]" /></Frow>
    <Frow v-if="kind === 'user'" :label="c('rules.user')"><Dropdown v-model="d.who" :options="users.map((u) => [u.name, u.full])" /></Frow>
    <Frow v-else-if="kind === 'group'" :label="c('rules.groupWord')">
      <Dropdown v-if="groups.length" v-model="d.who" :options="groups.map((g) => ['@' + g, g])" />
      <span v-else class="pill state">{{ c('users.noGroups') }}</span>
    </Frow>
  </div>
  <div class="well">
    <Frow :label="c('rules.pathOrPattern')" :tip="c('rules.pathTip')"><PathField id="a-rpath" v-model="d.path" :placeholder="c('rules.pathPh')" /></Frow>
    <Frow :label="c('rules.rule')" :tip="c('rules.ruleTip')"><Seg v-model="d.rule" :options="RULES.map(([r, g]) => [r, ruleName(r), g])" /></Frow>
  </div>
</template>
