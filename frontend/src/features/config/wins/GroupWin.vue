<script setup lang="ts">
import { FieldRow, SubCard, TextField, Toggle } from '@glimstone/vue';
import { onMounted, reactive, ref } from 'vue';
import { admin, type Group, type User } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { attempt, c, fail, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry; group: Group | null; users: User[]; groups: Group[]; saved: () => void }>();
const g = props.group, isNew = !g;
const name = ref(g?.name ?? ''), note = ref(g?.note ?? '');
const member = reactive(Object.fromEntries(props.users.map((u) => [u.name, !!g?.members.includes(u.name)])));
const nameEl = ref<InstanceType<typeof TextField> | null>(null);

async function save() {
  const n = name.value.trim();
  if (!/^[a-z0-9][a-z0-9_-]*$/.test(n)) return fail(n ? c('lowercaseOnly') : c('users.needGroupName'), nameEl.value?.input);
  if (props.groups.some((x) => x !== g && x.name === n)) return fail(c('users.groupTaken'), nameEl.value?.input);
  const body = { name: n, note: note.value.trim(), members: props.users.filter((u) => member[u.name]).map((u) => u.name) };
  if (!(await attempt(g ? admin.updateGroup(g.name, body) : admin.createGroup(body), nameEl.value?.input))) return;
  props.win.close();
  toast(isNew ? c('users.groupCreated', { name: n }) : c('saved'));
  props.saved();
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: isNew ? c('users.create') : c('save'), g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <SubCard>
    <FieldRow :label="c('users.groupName')"><TextField ref="nameEl" v-model="name" :placeholder="c('users.groupNamePh')" width="medium" /></FieldRow>
    <FieldRow :label="c('users.note')"><TextField v-model="note" :placeholder="c('users.notePh')" width="wide" /></FieldRow>
  </SubCard>
  <SubCard :title="c('users.members_')">
    <FieldRow v-for="u in users" :key="u.name" :label="u.full"><template #label>{{ u.full }} <small>{{ u.name }}</small></template><Toggle v-model="member[u.name]!" /></FieldRow>
  </SubCard>
</template>
