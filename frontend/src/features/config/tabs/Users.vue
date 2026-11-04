<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { admin, type Group, type Share, type User } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { useUser } from '@/stores/user';
import ARow from '../kit/ARow.vue';
import BareBtn from '../kit/BareBtn.vue';
import Btn from '../kit/Btn.vue';
import Card from '../kit/Card.vue';
import { ago, attempt, c, confirmWin, fail } from '../kit/kit';
import { roleName } from '../roles';
import GroupWin from '../wins/GroupWin.vue';
import UserWin from '../wins/UserWin.vue';

const users = ref<User[]>([]), groups = ref<Group[]>([]), shares = ref<Share[]>([]);
const myName = () => useUser().me.name;
async function load() {
  [users.value, groups.value] = await Promise.all([admin.users(), admin.groups()]);
  shares.value = await admin.shares(true).catch(() => []);
}
onMounted(() => { load().catch(() => {}); });

const initials = (u: User) => (u.full || u.name).split(/\s+/).map((x) => x[0]).join('').slice(0, 2).toUpperCase();
const admins = () => users.value.filter((u) => u.role === 'admin');
function userTip(u: User) {
  return [c('users.signsInAs', { name: u.name }), u.groups.length ? c('users.inGroups', { groups: u.groups.join(', ') }) : '', u.twofa ? c('users.withTwofa') : ''].filter(Boolean).join(' ');
}

const editUser = (u: User | null) => win({ title: u ? c('users.edit') : c('users.add'), body: UserWin, props: { user: u, users: users.value, groups: groups.value, saved: load } });
const editGroup = (g: Group | null) => win({ title: g ? c('users.editGroup') : c('users.addGroup'), body: GroupWin, props: { group: g, users: users.value, groups: groups.value, saved: load } });

function removeUser(u: User) {
  if (u.role === 'admin' && admins().length === 1) return fail(c('users.lastAdmin'));
  const n = shares.value.filter((s) => s.owner === u.name).length;
  const first = n ? c('users.removeTextShares', { full: u.full, shares: n === 1 ? c('users.theShare') : c('users.theShares', { n }), name: u.name }) : c('users.removeText', { full: u.full });
  confirmWin({
    title: c('users.remove'), verb: c('remove'), glyph: 'IconTrash', text: [first, c('users.filesStay')],
    run: async () => { if (!(await attempt(admin.deleteUser(u.name)))) return; await load(); toast(c('removed', { name: u.full })); },
  });
}
function removeGroup(g: Group) {
  confirmWin({
    title: c('users.removeGroup'), verb: c('remove'), glyph: 'IconTrash',
    text: [c('users.removeGroupText', { name: g.name }), g.members.length ? c('users.membersStay') : c('users.noMembers')],
    run: async () => { if (!(await attempt(admin.deleteGroup(g.name)))) return; await load(); toast(c('users.groupRemoved', { name: g.name })); },
  });
}
</script>

<template>
  <Card :title="c('users.users')">
    <div v-if="users.length" class="a-list">
      <ARow v-for="u in users" :key="u.name" :name="u.full" :tip="userTip(u)" data-uopen @click="editUser(u)">
        <template #tile><i>{{ initials(u) }}</i></template>
        <template #badges>
          <span v-if="u.name === myName()" class="pill state">{{ c('users.you') }}</span>
          <span class="pill">{{ roleName(u.role) }}</span><span class="pill mono">{{ u.scope }}</span>
          <span class="pill">{{ u.last ? ago(u.last) : c('users.never') }}</span>
        </template>
        <template #acts>
          <BareBtn g="IconPencil" :tip="c('edit')" @click="editUser(u)" />
          <BareBtn :g="u.name === myName() ? undefined : 'IconTrash'" :tip="c('remove')" @click="removeUser(u)" />
        </template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconAccounts" /><div>{{ c('users.nobody') }}</div></div>
    <div class="a-foot"><Btn :label="c('users.add')" g="IconAdd" @click="editUser(null)" /></div>
  </Card>
  <Card :title="c('users.groups')">
    <div v-if="groups.length" class="a-list">
      <ARow v-for="g in groups" :key="g.name" :name="g.name" :tip="g.note || null" data-gopen @click="editGroup(g)">
        <template #tile><G n="IconAccounts" /></template>
        <template #badges><span class="pill">{{ g.members.length === 1 ? c('users.oneMember') : c('users.members', { n: g.members.length }) }}</span></template>
        <template #acts>
          <BareBtn g="IconPencil" :tip="c('edit')" @click="editGroup(g)" />
          <BareBtn g="IconTrash" :tip="c('remove')" @click="removeGroup(g)" />
        </template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconAccounts" /><div>{{ c('users.noGroups') }}</div></div>
    <div class="a-foot"><Btn :label="c('users.addGroup')" g="IconAdd" @click="editGroup(null)" /></div>
  </Card>
</template>
