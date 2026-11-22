<script setup lang="ts">
import { Badge, Button, Card, EmptyState, ListRow } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { admin, type Group, type Share, type User } from '@/api/admin';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { useUser } from '@/stores/user';
import { ago, attempt, c, cn, confirmWin, fail } from '@/features/config/shared';
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

// The quiet line under a name, in the order the sign-in tile gives its accounts: role, area, last seen.
const userFacts = (u: User) => [roleName(u.role), u.scope, u.last ? ago(u.last) : c('users.never')].join(' · ');

const editUser = (u: User | null) => win({ title: u ? c('users.edit') : c('users.add'), body: UserWin, props: { user: u, users: users.value, groups: groups.value, saved: load } });
const editGroup = (g: Group | null) => win({ title: g ? c('users.editGroup') : c('users.addGroup'), body: GroupWin, props: { group: g, users: users.value, groups: groups.value, saved: load } });

function removeUser(u: User) {
  if (u.role === 'admin' && admins().length === 1) return fail(c('users.lastAdmin'));
  const n = shares.value.filter((s) => s.owner === u.name).length;
  const first = n ? c('users.removeTextShares', { full: u.full, shares: cn(n, 'users.theShares'), name: u.name }) : c('users.removeText', { full: u.full });
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
    <div v-if="users.length" class="a-rows">
      <ListRow v-for="(u, i) in users" :key="u.name" steady :name="u.full" :meta="userFacts(u)" :tip="userTip(u)" :data-uopen="i" @click="editUser(u)">
        <template #tile><i class="a-initials">{{ initials(u) }}</i></template>
        <template v-if="u.name === myName() || u.owner || u.blocked" #badges>
          <Badge v-if="u.name === myName()" size="small" tone="active">{{ c('users.you') }}</Badge>
          <Badge v-if="u.owner" size="small">{{ c('users.owner') }}</Badge>
          <Badge v-if="u.blocked" size="small" tone="warn">{{ c('users.blocked') }}</Badge>
        </template>
        <template #actions>
          <Button compact tone="subtle" :label="c('edit')" label-key="IconPencil" :data-uopen="i" @click.stop="editUser(u)" />
          <Button v-if="u.name !== myName() && !u.owner" compact tone="subtle" :label="c('remove')" label-key="IconTrash" @click.stop="removeUser(u)" />
        </template>
      </ListRow>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconAccounts')" :title="c('users.nobody')" />
    <div class="a-foot"><Button data-uadd :label="c('users.add')" label-key="IconAdd" @click="editUser(null)" /></div>
  </Card>
  <Card :title="c('users.groups')">
    <div v-if="groups.length" class="a-rows">
      <ListRow v-for="g in groups" :key="g.name" steady :name="g.name" :meta="cn(g.members.length, 'users.members')" :tip="g.note || null" data-gopen @click="editGroup(g)">
        <template #tile><G n="IconAccounts" /></template>
        <template #actions>
          <Button compact tone="subtle" :label="c('edit')" label-key="IconPencil" @click.stop="editGroup(g)" />
          <Button compact tone="subtle" :label="c('remove')" label-key="IconTrash" @click.stop="removeGroup(g)" />
        </template>
      </ListRow>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconAccounts')" :title="c('users.noGroups')" />
    <div class="a-foot"><Button data-gadd :label="c('users.addGroup')" label-key="IconAdd" @click="editGroup(null)" /></div>
  </Card>
</template>
