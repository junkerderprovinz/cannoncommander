<script setup lang="ts">
import { Button, Card } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { type Account, me, type Passkey, type Passkeys, type Session, type Token } from '@/api/admin';
import { fmtDate } from '@/core/format';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import ARow from '@/ui/ARow.vue';
import BareBtn from '@/ui/BareBtn.vue';
import Frow from '@/ui/Frow.vue';
import { ago, attempt, c, confirmWin, P } from '@/features/config/shared';
import Seg from '@/ui/Seg.vue';
import { device } from '../sessions';
import CodeWin from '../wins/CodeWin.vue';
import CodesWin from '../wins/CodesWin.vue';
import PasskeyWin from '../wins/PasskeyWin.vue';
import PasswordWin from '../wins/PasswordWin.vue';
import TokenWin from '../wins/TokenWin.vue';
import TotpWin from '../wins/TotpWin.vue';

const acc = ref<Account | null>(null);
const keys = ref<Passkeys>({ enabled: true, items: [] }), sessions = ref<Session[]>([]), tokens = ref<Token[]>([]);
async function load() {
  const [a, p, s, t] = await Promise.allSettled([me.account(), me.passkeys(), me.sessions(), me.tokens()]);
  if (a.status === 'fulfilled') acc.value = a.value;
  if (p.status === 'fulfilled') keys.value = p.value;
  if (s.status === 'fulfilled') sessions.value = s.value;
  if (t.status === 'fulfilled') tokens.value = t.value;
}
onMounted(() => { load().catch(() => {}); });
const others = computed(() => sessions.value.filter((s) => !s.current));
const passkeys = computed(() => keys.value.items);
// Some accounts sign in elsewhere or have their password kept by the admin, and then it is not changed here.
const ownPassword = computed(() => !!acc.value && acc.value.loginMethod === 'password' && !acc.value.lockPassword);

const password = () => win({ title: c('security.changePw'), body: PasswordWin, props: { done: load } });
const totpOn = () => win({ title: c('security.totpSetup'), wide: true, body: TotpWin, props: { done: load } });
const totpOff = () => win({ title: c('security.totpOff'), body: CodeWin, props: { verb: c('security.turnOff'), glyph: 'IconShieldOff', submit: async (code: string) => { await me.totpOff(code); await load(); toast(c('security.totpIsOff')); } } });
const newCodes = () => win({ title: c('security.newCodes'), body: CodeWin, props: { verb: c('security.regenerate'), glyph: 'IconRefresh', submit: async (code: string) => {
  const { codes } = await me.totpCodes(code);
  setTimeout(() => win({ title: c('security.newCodes'), body: CodesWin, props: { codes, done: async () => { await load(); toast(c('security.codesNew')); } } }), 0);
} } });
const addPasskey = () => win({ title: c('security.addPasskey'), body: PasskeyWin, props: { taken: passkeys.value.map((p) => p.name), done: load } });
const addToken = () => win({ title: c('security.addToken'), body: TokenWin, props: { taken: tokens.value.map((t) => t.name), done: load } });

function removePasskey(p: Passkey) {
  confirmWin({ title: c('security.removePasskey'), verb: c('remove'), glyph: 'IconTrash', text: [c('security.removePasskeyText', { name: p.name })],
    run: async () => { if (!(await attempt(me.deletePasskey(p.id)))) return; await load(); toast(c('security.passkeyRemoved', { name: p.name })); } });
}
async function signOut(s: Session) {
  await me.endSession(s.id);
  await load();
  toast(c('security.signedOut', { device: device(s.userAgent).name }));
}
function signOutAll() {
  const n = others.value.length;
  confirmWin({ title: c('security.signOutAll'), verb: c('security.signOut'), glyph: 'IconSignOut',
    text: [n === 1 ? c('security.signOutOne') : c('security.signOutMany', { n }), c('security.tokensStay')],
    run: async () => { for (const s of others.value) await me.endSession(s.id); await load(); toast(c('security.allSignedOut')); } });
}
function revoke(t: Token) {
  confirmWin({ title: c('security.revokeToken'), verb: c('security.revoke'), glyph: 'IconKeyRevoke', text: [c('security.revokeText', { name: t.name })],
    run: async () => { if (!(await attempt(me.deleteToken(t.id)))) return; await load(); toast(c('security.tokenRevoked', { name: t.name })); } });
}
const tokenTip = (t: Token) => c('security.tokenCreated', { date: fmtDate(t.created) }) + (t.expires ? c('security.tokenExpires', { date: fmtDate(t.expires) }) : c('security.tokenNoExpiry'));
</script>

<template>
  <Card :title="c('security.signIn')">
    <Frow :label="c('users.password')" :tip="ownPassword || !acc ? c('security.pwTip') : c('security.pwLocked')"><span class="a-ctl"><span class="pill">{{ acc?.pwChanged ? c('security.changedAgo', { ago: ago(acc.pwChanged) }) : c('users.pwSet') }}</span><Button v-if="ownPassword" data-pw :label="c('security.change')" label-key="IconKey" @click="password" /></span></Frow>
    <Frow :label="c('security.stay')" :tip="c('security.stayTip')"><Seg id="a-stay" v-model="P.stayDays" :options="[[1, c('security.day1')], [7, c('general.days', { n: 7 })], [30, c('general.days', { n: 30 })]]" /></Frow>
  </Card>
  <Card :title="c('security.twofa')">
    <template v-if="acc?.twofa">
      <Frow :label="c('security.status')" :tip="c('security.onTip')"><span class="a-ctl"><span class="pill ok">{{ c('security.on') }}</span><Button data-2off :label="c('security.turnOff')" label-key="IconShieldOff" @click="totpOff" /></span></Frow>
      <Frow :label="c('security.codes')" :tip="c('security.codesTip')"><span class="a-ctl"><span class="pill" :class="{ warn: acc.recovery < 3 }">{{ c('security.codesLeft', { n: acc.recovery }) }}</span><Button data-2codes :label="c('security.regenerate')" label-key="IconRefresh" @click="newCodes" /></span></Frow>
    </template>
    <Frow v-else :label="c('security.status')" :tip="c('security.offTip')"><span class="a-ctl"><span class="pill state">{{ c('security.off') }}</span><Button data-2on :label="c('security.setUp')" label-key="IconShieldOn" @click="totpOn" /></span></Frow>
  </Card>
  <Card :title="c('security.passkeys')">
    <div v-if="passkeys.length" class="a-list">
      <ARow v-for="p in passkeys" :key="p.id" :name="p.name" :tip="c('security.passkeyTip', { origin: p.origin })">
        <template #tile><G n="IconKey" /></template>
        <template #badges><span class="pill mono">{{ p.origin }}</span><span class="pill">{{ p.last ? c('security.used', { ago: ago(p.last) }) : c('security.neverUsed') }}</span></template>
        <template #acts><BareBtn g="IconTrash" :tip="c('remove')" @click="removePasskey(p)" /></template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconKey" /><div>{{ keys.enabled ? c('security.noPasskey') : c('security.passkeysOff') }}</div></div>
    <div v-if="keys.enabled" class="a-foot"><Button data-pkadd :label="c('security.addPasskey')" label-key="IconAdd" @click="addPasskey" /></div>
  </Card>
  <Card :title="c('security.sessions')">
    <div v-if="sessions.length" class="a-list">
      <ARow v-for="s in sessions" :key="s.id" :name="device(s.userAgent).name">
        <template #tile><G :n="device(s.userAgent).g" /></template>
        <template #badges><span class="pill mono">{{ s.ip }}</span><span v-if="s.current" class="pill state">{{ c('security.thisSession') }}</span><span v-else class="pill">{{ ago(s.seen) }}</span></template>
        <template #acts><BareBtn :g="s.current ? undefined : 'IconSignOut'" :tip="c('security.signOut')" @click="signOut(s)" /></template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconBrowser" /><div>{{ c('security.noSessions') }}</div></div>
    <div v-if="others.length" class="a-foot"><Button data-soutall :label="c('security.signOutAll')" label-key="IconSignOut" @click="signOutAll" /></div>
  </Card>
  <Card :title="c('security.tokens')">
    <div v-if="tokens.length" class="a-list">
      <ARow v-for="t in tokens" :key="t.id" :name="t.name" :tip="tokenTip(t)">
        <template #tile><G n="IconKey" /></template>
        <template #badges><span class="pill">{{ t.rights === 'rw' ? c('security.readWrite') : c('security.read') }}</span><span class="pill">{{ t.last ? c('security.used', { ago: ago(t.last) }) : c('security.neverUsed') }}</span></template>
        <template #acts><BareBtn g="IconKeyRevoke" :tip="c('security.revoke')" @click="revoke(t)" /></template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconKey" /><div>{{ c('security.noToken') }}</div></div>
    <div class="a-foot"><Button data-tadd :label="c('security.addToken')" label-key="IconAdd" @click="addToken" /></div>
  </Card>
</template>
