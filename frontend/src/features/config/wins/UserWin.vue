<script setup lang="ts">
import { PasswordField } from '@glimstone/vue';
import { nextTick, onMounted, reactive, ref } from 'vue';
import { admin, exists, type Group, type Role, type User } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { useUser } from '@/stores/user';
import Frow from '@/ui/Frow.vue';
import { attempt, c, fail, stepFoot } from '@/features/config/shared';
import PathField from '@/ui/PathField.vue';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';
import { eyeWords } from '@/ui/kit';
import { ROLES, roleName } from '../roles';

const props = defineProps<{ win: WinEntry; user: User | null; users: User[]; groups: Group[]; saved: () => void }>();
const u = props.user, isNew = !u;
const self = !!u && u.name === useUser().me.name;
const d = reactive({ name: u?.name ?? '', full: u?.full ?? '', role: (u?.role ?? 'user') as Role, scope: u?.scope ?? '/mnt/user' });
const inGroup = reactive(Object.fromEntries(props.groups.map((g) => [g.name, !!u?.groups.includes(g.name)])));
const pwOpen = ref(isNew), pw1 = ref(''), pw2 = ref('');
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;
const admins = () => props.users.filter((x) => x.role === 'admin');

async function openPassword() {
  pwOpen.value = true;
  await nextTick();
  (at('#a-upw') as HTMLInputElement | null)?.focus();
}

async function save() {
  const name = d.name.trim(), scope = d.scope.trim();
  if (!/^[a-z][a-z0-9._-]{1,31}$/.test(name)) return fail(name ? c('users.nameRule') : c('users.needName'), at('#a-uname'));
  if (props.users.some((x) => x !== u && x.name === name)) return fail(c('users.nameTaken'), at('#a-uname'));
  if (!(await exists(scope))) return fail(c('noFolder'), at('#a-uscope'));
  if (self && d.role !== 'admin') return fail(c('users.ownAdmin'), at('#a-urole'));
  if (u && u.role === 'admin' && d.role !== 'admin' && admins().length === 1) return fail(c('users.needAdmin'), at('#a-urole'));
  if (pwOpen.value) {
    if (pw1.value.length < 12) return fail(c('users.pwShort'), at('#a-upw'));
    if (pw1.value !== pw2.value) return fail(c('users.pwDiffer'), at('#a-upw2'));
  }
  const groups = props.groups.filter((g) => inGroup[g.name]).map((g) => g.name);
  const body = { name, full: d.full.trim() || name, role: d.role, scope, groups };
  const done = await attempt(u ? admin.updateUser(u.name, { ...body, ...(pwOpen.value ? { password: pw1.value } : {}) }) : admin.createUser({ ...body, password: pw1.value }), at('#a-uname'));
  if (!done) return;
  props.win.close();
  toast(isNew ? c('users.created', { name: body.full }) : pwOpen.value ? c('users.savedPw') : c('saved'));
  props.saved();
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: isNew ? c('users.create') : c('save'), g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div class="well glim-well">
    <Frow :label="c('users.loginName')" :tip="c('users.loginNameTip')"><input id="a-uname" v-model="d.name" class="glim-input m" :readonly="self" :placeholder="c('users.loginNamePh')"></Frow>
    <Frow :label="c('users.displayName')"><input id="a-ufull" v-model="d.full" class="glim-input m" :placeholder="c('users.displayNamePh')"></Frow>
  </div>
  <div class="well glim-well">
    <Frow :label="c('users.role')" :tip="c('users.roleTip')"><Seg id="a-urole" v-model="d.role" :options="ROLES.map((r) => [r, roleName(r)])" /></Frow>
    <Frow :label="c('users.scope')" :tip="c('users.scopeTip')"><PathField id="a-uscope" v-model="d.scope" /></Frow>
  </div>
  <div class="well glim-well">
    <div class="eyebrow">{{ c('users.groups') }}</div>
    <Frow v-for="g in groups" :key="g.name" :label="g.name" :tip="g.note"><Sw v-model="inGroup[g.name]!" :data-ug="g.name" /></Frow>
    <div v-if="!groups.length" class="muted">{{ c('users.noGroups') }}</div>
  </div>
  <div v-if="!self" class="well glim-well">
    <template v-if="pwOpen">
      <Frow :label="c('users.password')" :tip="c('users.pwTip')"><PasswordField id="a-upw" v-model="pw1" class="m" autocomplete="new-password" :text="eyeWords()" /></Frow>
      <Frow :label="c('users.repeat')"><PasswordField id="a-upw2" v-model="pw2" class="m" autocomplete="new-password" :text="eyeWords()" /></Frow>
    </template>
    <Frow v-else :label="c('users.password')" :tip="self ? c('users.ownPwTip') : c('users.newPwTip')">
      <span class="a-ctl"><span class="pill">{{ c('users.pwSet') }}</span><button class="btn" @click="openPassword"><G n="IconKey" />{{ c('users.pwNew') }}</button></span>
    </Frow>
  </div>
</template>
