<script setup lang="ts">
import { segItems } from '@/ui/kit';
import { Button, Card, FieldRow, IconButton, ListRow, Pill, Selector } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { type Account, me, type Passkey, type Passkeys, type Session, type Token } from '@/api/admin';
import { fmtDate } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { ago, attempt, c, confirmWin, P } from '@/features/config/shared';
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
    <FieldRow :label="c('users.password')" :hint="ownPassword || !acc ? c('security.pwTip') : c('security.pwLocked')"><span class="a-ctl"><Pill>{{ acc?.pwChanged ? c('security.changedAgo', { ago: ago(acc.pwChanged) }) : c('users.pwSet') }}</Pill><Button v-if="ownPassword" data-pw :label="c('security.change')" label-key="IconKey" @click="password" /></span></FieldRow>
    <FieldRow :label="c('security.stay')" :hint="c('security.stayTip')"><Selector id="a-stay" v-model="P.stayDays" :items="segItems([[1, c('security.day1')], [7, c('general.days', { n: 7 })], [30, c('general.days', { n: 30 })]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
  </Card>
  <Card :title="c('security.twofa')">
    <template v-if="acc?.twofa">
      <FieldRow :label="c('security.status')" :hint="c('security.onTip')"><span class="a-ctl"><Pill tone="ok">{{ c('security.on') }}</Pill><Button data-2off :label="c('security.turnOff')" label-key="IconShieldOff" @click="totpOff" /></span></FieldRow>
      <FieldRow :label="c('security.codes')" :hint="c('security.codesTip')"><span class="a-ctl"><Pill :tone="acc.recovery < 3 ? 'warn' : 'neutral'">{{ c('security.codesLeft', { n: acc.recovery }) }}</Pill><Button data-2codes :label="c('security.regenerate')" label-key="IconRefresh" @click="newCodes" /></span></FieldRow>
    </template>
    <FieldRow v-else :label="c('security.status')" :hint="c('security.offTip')"><span class="a-ctl"><Pill tone="state">{{ c('security.off') }}</Pill><Button data-2on :label="c('security.setUp')" label-key="IconShieldOn" @click="totpOn" /></span></FieldRow>
  </Card>
  <Card :title="c('security.passkeys')">
    <div v-if="passkeys.length" class="a-list">
      <ListRow v-for="p in passkeys" :key="p.id" :name="p.name" :tip="c('security.passkeyTip', { origin: p.origin })">
        <template #tile><G n="IconKey" /></template>
        <template #badges><Pill class="mono">{{ p.origin }}</Pill><Pill>{{ p.last ? c('security.used', { ago: ago(p.last) }) : c('security.neverUsed') }}</Pill></template>
        <template #actions><IconButton :glyph="glyphComponent('IconTrash')" :tip="c('remove')" @click="removePasskey(p)" /></template>
      </ListRow>
    </div>
    <div v-else class="empty"><G n="IconKey" /><div>{{ keys.enabled ? c('security.noPasskey') : c('security.passkeysOff') }}</div></div>
    <div v-if="keys.enabled" class="a-foot"><Button data-pkadd :label="c('security.addPasskey')" label-key="IconAdd" @click="addPasskey" /></div>
  </Card>
  <Card :title="c('security.sessions')">
    <div v-if="sessions.length" class="a-list">
      <ListRow v-for="s in sessions" :key="s.id" :name="device(s.userAgent).name">
        <template #tile><G :n="device(s.userAgent).g" /></template>
        <template #badges><Pill class="mono">{{ s.ip }}</Pill><Pill v-if="s.current" tone="state">{{ c('security.thisSession') }}</Pill><Pill v-else>{{ ago(s.seen) }}</Pill></template>
        <template #actions><IconButton v-if="!s.current" :glyph="glyphComponent('IconSignOut')" :tip="c('security.signOut')" @click="signOut(s)" /><span v-else class="glim-icon-btn-gap"></span></template>
      </ListRow>
    </div>
    <div v-else class="empty"><G n="IconBrowser" /><div>{{ c('security.noSessions') }}</div></div>
    <div v-if="others.length" class="a-foot"><Button data-soutall :label="c('security.signOutAll')" label-key="IconSignOut" @click="signOutAll" /></div>
  </Card>
  <Card :title="c('security.tokens')">
    <div v-if="tokens.length" class="a-list">
      <ListRow v-for="t in tokens" :key="t.id" :name="t.name" :tip="tokenTip(t)">
        <template #tile><G n="IconKey" /></template>
        <template #badges><Pill>{{ t.rights === 'rw' ? c('security.readWrite') : c('security.read') }}</Pill><Pill>{{ t.last ? c('security.used', { ago: ago(t.last) }) : c('security.neverUsed') }}</Pill></template>
        <template #actions><IconButton :glyph="glyphComponent('IconKeyRevoke')" :tip="c('security.revoke')" @click="revoke(t)" /></template>
      </ListRow>
    </div>
    <div v-else class="empty"><G n="IconKey" /><div>{{ c('security.noToken') }}</div></div>
    <div class="a-foot"><Button data-tadd :label="c('security.addToken')" label-key="IconAdd" @click="addToken" /></div>
  </Card>
</template>
