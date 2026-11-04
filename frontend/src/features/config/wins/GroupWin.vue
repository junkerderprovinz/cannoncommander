<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { admin, type Group, type User } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '../kit/Frow.vue';
import { c, fail, setFoot } from '../kit/kit';
import Sw from '../kit/Sw.vue';

const props = defineProps<{ win: WinEntry; group: Group | null; users: User[]; groups: Group[]; saved: () => void }>();
const g = props.group, isNew = !g;
const name = ref(g?.name ?? ''), note = ref(g?.note ?? '');
const member = reactive(Object.fromEntries(props.users.map((u) => [u.name, !!g?.members.includes(u.name)])));
const nameEl = ref<HTMLInputElement | null>(null);

async function save() {
  const n = name.value.trim();
  if (!/^[a-z0-9][a-z0-9_-]*$/.test(n)) return fail(n ? c('lowercaseOnly') : c('users.needGroupName'), nameEl.value);
  if (props.groups.some((x) => x !== g && x.name === n)) return fail(c('users.groupTaken'), nameEl.value);
  const body = { name: n, note: note.value.trim(), members: props.users.filter((u) => member[u.name]).map((u) => u.name) };
  if (g) await admin.updateGroup(g.name, body);
  else await admin.createGroup(body);
  props.win.close();
  toast(isNew ? c('users.groupCreated', { name: n }) : c('saved'));
  props.saved();
}

onMounted(() => setFoot(props.win, [cancelBtn(), { label: isNew ? c('users.create') : c('save'), g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div class="well">
    <Frow :label="c('users.groupName')"><input ref="nameEl" v-model="name" class="field m" :placeholder="c('users.groupNamePh')"></Frow>
    <Frow :label="c('users.note')"><input v-model="note" class="field w" :placeholder="c('users.notePh')"></Frow>
  </div>
  <div class="well">
    <div class="eyebrow">{{ c('users.members_') }}</div>
    <Frow v-for="u in users" :key="u.name"><template #label>{{ u.full }} <small>{{ u.name }}</small></template><Sw v-model="member[u.name]!" /></Frow>
  </div>
</template>
