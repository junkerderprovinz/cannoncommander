<script setup lang="ts">
// The ways into another person's account, each one to take away on its own or all of them at once. The password
// stays, that is changed in the user's own window.
import { Button, EmptyState, IconButton, ListRow, Pill, SubCard } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { admin, type Passkey, type Session, type Token, type User } from '@/api/admin';
import { fmtDate } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { attempt, ago, c, confirmWin } from '@/features/config/shared';
import { device } from '../sessions';

const props = defineProps<{ user: User; changed: () => void }>();
const who = props.user.full || props.user.name;
const sessions = ref<Session[]>([]), tokens = ref<Token[]>([]), passkeys = ref<Passkey[]>([]);
const any = computed(() => sessions.value.length + tokens.value.length + passkeys.value.length > 0);

async function load() {
  const [s, t, p] = await Promise.allSettled([admin.userSessions(props.user.name), admin.userTokens(props.user.name), admin.userPasskeys(props.user.name)]);
  if (s.status === 'fulfilled') sessions.value = s.value;
  if (t.status === 'fulfilled') tokens.value = t.value;
  if (p.status === 'fulfilled') passkeys.value = p.value;
}
onMounted(() => { load().catch(() => {}); });

async function after(done: Promise<unknown>, note: string) {
  if (!(await attempt(done))) return;
  await load();
  props.changed();
  toast(note);
}
function signOut(s: Session) {
  const name = device(s.userAgent).name;
  confirmWin({ title: c('access.signOut'), verb: c('security.signOut'), glyph: 'IconSignOut', text: [c('access.signOutText', { device: name, who })],
    run: () => after(admin.endUserSession(props.user.name, s.id), c('security.signedOut', { device: name })) });
}
function revoke(t: Token) {
  confirmWin({ title: c('security.revokeToken'), verb: c('security.revoke'), glyph: 'IconKeyRevoke', text: [c('security.revokeText', { name: t.name })],
    run: () => after(admin.revokeUserToken(props.user.name, t.id), c('security.tokenRevoked', { name: t.name })) });
}
function removePasskey(p: Passkey) {
  confirmWin({ title: c('security.removePasskey'), verb: c('remove'), glyph: 'IconTrash', text: [c('access.removePasskeyText', { name: p.name, who })],
    run: () => after(admin.removeUserPasskey(props.user.name, p.id), c('security.passkeyRemoved', { name: p.name })) });
}
function endAll() {
  confirmWin({ title: c('access.endAll'), verb: c('access.end'), glyph: 'IconKeyRevoke', text: [c('access.endAllText', { who }), c('access.passwordStays')],
    run: () => after(admin.updateUser(props.user.name, { revoke: true }), c('access.allEnded', { who })) });
}
const tokenTip = (t: Token) => c('security.tokenCreated', { date: fmtDate(t.created) }) + (t.expires ? c('security.tokenExpires', { date: fmtDate(t.expires) }) : c('security.tokenNoExpiry'));
</script>

<template>
  <SubCard :title="c('security.sessions')">
    <div v-if="sessions.length" class="a-list">
      <ListRow v-for="s in sessions" :key="s.id" :name="device(s.userAgent).name" :tip="c('access.since', { date: fmtDate(s.created) })">
        <template #tile><G :n="device(s.userAgent).g" /></template>
        <template #badges><Pill class="mono">{{ s.ip }}</Pill><Pill>{{ ago(s.seen) }}</Pill></template>
        <template #actions><IconButton :glyph="glyphComponent('IconSignOut')" :tip="c('security.signOut')" data-acc-session @click="signOut(s)" /></template>
      </ListRow>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconBrowser')" :title="c('access.noSessions')" />
  </SubCard>
  <SubCard :title="c('security.tokens')">
    <div v-if="tokens.length" class="a-list">
      <ListRow v-for="t in tokens" :key="t.id" :name="t.name" :tip="tokenTip(t)">
        <template #tile><G n="IconKey" /></template>
        <template #badges><Pill>{{ t.rights === 'rw' ? c('security.readWrite') : c('security.read') }}</Pill><Pill>{{ t.last ? c('security.used', { ago: ago(t.last) }) : c('security.neverUsed') }}</Pill></template>
        <template #actions><IconButton :glyph="glyphComponent('IconKeyRevoke')" :tip="c('security.revoke')" data-acc-token @click="revoke(t)" /></template>
      </ListRow>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconKey')" :title="c('access.noTokens')" />
  </SubCard>
  <SubCard :title="c('security.passkeys')">
    <div v-if="passkeys.length" class="a-list">
      <ListRow v-for="p in passkeys" :key="p.id" :name="p.name" :tip="c('security.passkeyTip', { origin: p.origin })">
        <template #tile><G n="IconKey" /></template>
        <template #badges><Pill class="mono">{{ p.origin }}</Pill><Pill>{{ p.last ? c('security.used', { ago: ago(p.last) }) : c('security.neverUsed') }}</Pill></template>
        <template #actions><IconButton :glyph="glyphComponent('IconTrash')" :tip="c('remove')" data-acc-passkey @click="removePasskey(p)" /></template>
      </ListRow>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconKey')" :title="c('access.noPasskeys')" />
  </SubCard>
  <div v-if="any" class="a-foot"><Button data-acc-all :label="c('access.endAll')" label-key="IconKeyRevoke" @click="endAll" /></div>
</template>
