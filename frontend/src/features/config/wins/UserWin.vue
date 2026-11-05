<script setup lang="ts">
import { pathField } from '@/ui/picker';
import { Button, FieldRow, FieldRowGroup, PasswordField, PathField, Pill, Selector, SubCard, TextField, Toggle } from '@glimstone/vue';
import { nextTick, onMounted, reactive, ref } from 'vue';
import { admin, exists, type Group, type Role, type User } from '@/api/admin';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { useUser } from '@/stores/user';
import { attempt, c, fail, stepFoot } from '@/features/config/shared';
import { eyeWords, segItems } from '@/ui/kit';
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
  <SubCard>
    <FieldRow :label="c('users.loginName')" :hint="c('users.loginNameTip')"><TextField id="a-uname" v-model="d.name" :readonly="self" :placeholder="c('users.loginNamePh')" width="medium" /></FieldRow>
    <FieldRow :label="c('users.displayName')"><TextField id="a-ufull" v-model="d.full" :placeholder="c('users.displayNamePh')" width="medium" /></FieldRow>
  </SubCard>
  <SubCard>
    <FieldRowGroup>
      <FieldRow :label="c('users.role')" :hint="c('users.roleTip')"><Selector id="a-urole" v-model="d.role" :items="segItems(ROLES.map((r) => [r, roleName(r)]))" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
      <FieldRow :label="c('users.scope')" :hint="c('users.scopeTip')"><PathField v-model="d.scope" v-bind="pathField('a-uscope')" /></FieldRow>
    </FieldRowGroup>
  </SubCard>
  <SubCard :title="c('users.groups')">
    <FieldRow v-for="g in groups" :key="g.name" :label="g.name" :hint="g.note"><Toggle v-model="inGroup[g.name]!" :data-ug="g.name" /></FieldRow>
    <div v-if="!groups.length" class="muted">{{ c('users.noGroups') }}</div>
  </SubCard>
  <SubCard v-if="!self">
    <template v-if="pwOpen">
      <FieldRow :label="c('users.password')" :hint="c('users.pwTip')"><PasswordField id="a-upw" v-model="pw1" width="medium" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
      <FieldRow :label="c('users.repeat')"><PasswordField id="a-upw2" v-model="pw2" width="medium" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
    </template>
    <FieldRow v-else :label="c('users.password')" :hint="self ? c('users.ownPwTip') : c('users.newPwTip')">
      <Pill>{{ c('users.pwSet') }}</Pill><Button :label="c('users.pwNew')" label-key="IconKey" @click="openPassword" />
    </FieldRow>
  </SubCard>
</template>
