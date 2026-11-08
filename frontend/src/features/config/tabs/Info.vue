<script setup lang="ts">
import { Button, Card, FieldRow, FieldRowGroup, Toggle } from '@glimstone/vue';
import { GLIMSTONE_VERSION } from '@glimstone/core';
import { computed, onMounted, ref, watch } from 'vue';
import { admin, type AppSettings, type System, type UpdateCheck } from '@/api/admin';
import { toast } from '@/core/toast';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { win } from '@/core/windows';
import { NEWS, unseen } from '../help';
import { c } from '@/features/config/shared';
import { bugReport, MAIL, REPO, versionLink } from '../support';
import CoffeeWin from '../wins/CoffeeWin.vue';
import CryptoWin from '../wins/CryptoWin.vue';
import PaypalWin from '../wins/PaypalWin.vue';

const sys = ref<System | null>(null), app = ref<AppSettings | null>(null);
const update = ref<'idle' | 'busy' | 'ok' | 'newer'>('idle');
const latest = ref<UpdateCheck | null>(null);
onMounted(() => {
  admin.system().then((s) => { sys.value = s; }, () => {});
  admin.app().then((a) => { app.value = a; }, () => {});
});
const saveUpdates = (change: Partial<AppSettings['updates']>) => {
  Object.assign(app.value!.updates, change);
  admin.saveApp({ updates: change }).then(() => toast(c('saved')), () => toast(c('notSaved'), 'warn'));
};
const autoCheck = computed({ get: () => !!app.value?.updates.autoCheck, set: (v: boolean) => saveUpdates({ autoCheck: v }) });
// Only the desktop app can put a new version in place itself.
const desktop = computed(() => sys.value?.deployment === 'desktop');
const autoUpdate = computed({ get: () => desktop.value && !!app.value?.updates.autoUpdate, set: (v: boolean) => { if (desktop.value) saveUpdates({ autoUpdate: v }); } });

// A version number is a plain link to its release page, as in every app of the workshop.
const plain = (v?: string) => v?.replace(/^v/, '');
const rclone = computed(() => plain(sys.value?.tools.rclone?.version));
const versions = computed(() => [
  { name: 'CannonCommander', v: plain(sys.value?.version), href: sys.value ? versionLink(sys.value) : undefined },
  { name: 'GlimStone', v: plain(GLIMSTONE_VERSION), href: 'https://github.com/junkerderprovinz/glimstone/releases/tag/v' + plain(GLIMSTONE_VERSION), tip: c('info.glimstoneTip') },
  { name: 'rclone', v: rclone.value, href: 'https://github.com/rclone/rclone/releases/tag/v' + rclone.value, tip: c('info.rcloneTip') },
]);
const n = computed(() => unseen());

// A check nobody pressed for stays quiet when GitHub does not answer.
async function check(quiet = false) {
  update.value = 'busy';
  const got = await admin.checkUpdate().catch(() => null);
  latest.value = got;
  update.value = !got ? 'idle' : got.updateAvailable ? 'newer' : 'ok';
  if (!got && !quiet) toast(c('app.noAnswer'), 'warn');
}
// Once as the page opens while the switch is on, and once more when it is turned on.
watch(() => app.value?.updates.autoCheck, (on) => { if (on) void check(true); });
const open = (url: string) => window.open(url, '_blank', 'noopener');
// Subject only: a prefilled body reads as a form to fill in.
const mail = () => open(`mailto:${MAIL}?subject=${encodeURIComponent(c('info.mailSubject'))}`);

const readme = (tile: string, mark: string, glyph: string, name: string, sub: string | undefined, go: () => unknown, aria?: string, id?: string) => ({ tile, mark, glyph, name, sub, go, aria, id });
const help = computed(() => [
  readme('glim-tile-github', 'glim-github-mark', 'IconGithub', 'GitHub', undefined, () => open(REPO)),
  readme('glim-tile-house', 'glim-house-mark', 'IconMail', c('info.mail'), undefined, mail),
  readme('glim-tile-docs', 'glim-docs-mark', 'IconManual', c('info.docs'), c('info.open'), () => run('docs')),
  readme('glim-tile-github', 'glim-github-mark', 'IconDiagnostics', c('info.bug'), c('info.download'), bugReport),
]);
const give = computed(() => [
  readme('glim-tile-coffee', 'a-coffee-mark', 'IconCoffee', c('info.coffee'), undefined, () => win({ title: c('info.coffeeTitle'), body: CoffeeWin }), c('info.coffeeTitle'), 'a-coffee'),
  readme('glim-tile-paypal', 'glim-paypal-mark', 'IconPayPal', 'PayPal', undefined, () => win({ title: 'PayPal', body: PaypalWin }), undefined, 'a-paypal'),
  readme('glim-tile-bitcoin', 'glim-bitcoin-mark', 'IconBitcoin', c('info.crypto'), undefined, () => win({ title: c('info.crypto'), body: CryptoWin }), undefined, 'a-crypto'),
]);
</script>

<template>
  <Card :title="c('info.version')" :hint="c('info.versionHint')">
    <FieldRow v-for="x in versions" :key="x.name" :label="x.name" :hint="x.tip">
      <a v-if="x.v" class="a-vlink glim-num" :href="x.href" target="_blank" rel="noopener">{{ x.v }}</a>
    </FieldRow>
    <div class="a-foot">
      <Button :label="n ? c('info.newN', { n }) : c('info.new')" label-key="IconWhatsNew" data-run="whatsNew" @click="run('whatsNew')"><template #before><span class="a-dot" :class="{ seen: !n }"></span></template></Button>
      <Button v-if="latest?.ready" id="a-update" :label="c('app.ready', { v: plain(latest.ready)! })" label-key="IconWhatsNew" tone="ok" @click="admin.restart()" />
      <Button v-else-if="update === 'newer'" id="a-update" :label="c('app.newer', { v: latest!.latest })" label-key="IconWhatsNew" :href="latest?.releaseUrl || REPO + '/releases'" target="_blank" rel="noopener" />
      <Button v-else id="a-update" :label="update === 'ok' ? c('info.current') : c('app.check')" :label-key="update === 'ok' ? 'IconCheck' : 'IconSearch'" :tone="update === 'ok' ? 'ok' : 'neutral'" :disabled="update === 'busy'" :title="update === 'ok' ? c('app.check') : undefined" @click="check()" />
    </div>
  </Card>
  <Card v-if="app" :title="c('app.updates')">
    <FieldRowGroup>
      <FieldRow :label="c('app.autoCheck')" :hint="c(desktop ? 'app.autoCheckDesktopTip' : 'app.autoCheckTip')"><Toggle v-model="autoCheck" /></FieldRow>
      <!-- A container cannot replace itself, so there this one stays off and says why. -->
      <FieldRow :label="c('app.autoUpdate')" :hint="c(desktop ? 'app.autoUpdateDesktopTip' : 'app.autoUpdateTip')"><Toggle v-model="autoUpdate" :disabled="!desktop" /></FieldRow>
    </FieldRowGroup>
  </Card>
  <Card :title="c('info.help')" :hint="c('info.helpHint')">
    <div class="a-about">
      <p>{{ c('info.helpText') }}</p>
      <div class="glim-readme-btn-rows">
        <div v-for="b in help" :key="b.name" class="glim-readme-btn-unit group" :class="b.tile"><button class="glim-readme-btn glim-brand-tile" @click="b.go"><span class="glim-readme-btn-mark" :class="b.mark"><G :n="b.glyph" /></span><span class="glim-readme-btn-text"><span class="glim-readme-btn-name">{{ b.name }}</span><span v-if="b.sub" class="glim-readme-btn-sub">{{ b.sub }}</span></span></button></div>
      </div>
    </div>
  </Card>
  <Card :title="c('info.about')">
    <div class="a-about">
      <p>{{ c('info.about1') }}</p>
      <p>{{ c('info.about2') }}</p>
      <div class="glim-readme-btn-rows glim-about-give">
        <div v-for="b in give" :key="b.name" class="glim-readme-btn-unit group" :class="b.tile"><button :id="b.id" class="glim-readme-btn glim-brand-tile" :aria-label="b.aria" @click="b.go"><span class="glim-readme-btn-mark" :class="b.mark"><G :n="b.glyph" /></span><span class="glim-readme-btn-text"><span class="glim-readme-btn-name">{{ b.name }}</span></span></button></div>
      </div>
    </div>
  </Card>
</template>
