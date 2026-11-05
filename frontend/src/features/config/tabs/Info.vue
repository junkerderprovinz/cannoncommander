<script setup lang="ts">
import { Button, Card, FieldRow } from '@glimstone/vue';
import { GLIMSTONE_VERSION } from '@glimstone/core';
import { computed, onMounted, ref } from 'vue';
import { admin, type System, type UpdateCheck } from '@/api/admin';
import { toast } from '@/core/toast';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { win } from '@/core/windows';
import { NEWS, unseen } from '../help';
import { c } from '@/features/config/shared';
import { bugReport, MAIL, REPO } from '../support';
import CoffeeWin from '../wins/CoffeeWin.vue';
import CryptoWin from '../wins/CryptoWin.vue';
import PaypalWin from '../wins/PaypalWin.vue';

const sys = ref<System | null>(null);
const update = ref<'idle' | 'busy' | 'ok' | 'newer'>('idle');
const latest = ref<UpdateCheck | null>(null);
onMounted(() => { admin.system().then((s) => { sys.value = s; }, () => {}); });

// A version number is a plain link to its release page, as in every app of the workshop.
const versions = computed(() => [
  { name: 'CannonCommander', v: sys.value?.version, href: REPO + '/releases/tag/v' },
  { name: 'GlimStone', v: GLIMSTONE_VERSION, href: 'https://github.com/junkerderprovinz/glimstone/releases/tag/v', tip: c('info.glimstoneTip') },
  { name: 'rclone', v: sys.value?.tools.rclone?.version, href: 'https://github.com/rclone/rclone/releases/tag/v', tip: c('info.rcloneTip') },
]);
const n = computed(() => unseen());

async function check() {
  update.value = 'busy';
  const got = await admin.checkUpdate().catch(() => null);
  latest.value = got;
  update.value = !got ? 'idle' : got.updateAvailable ? 'newer' : 'ok';
  if (!got) toast(c('app.noAnswer'), 'warn');
}
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
      <a v-if="x.v" class="a-vlink glim-num" :href="x.href + x.v" target="_blank" rel="noopener">{{ String(x.v).replace(/^v/, '') }}</a>
    </FieldRow>
    <div class="a-foot">
      <Button :label="n ? c('info.newN', { n }) : c('info.new')" label-key="IconWhatsNew" data-run="whatsNew" @click="run('whatsNew')"><template #before><span class="a-dot" :class="{ seen: !n }"></span></template></Button>
      <Button v-if="update === 'newer'" id="a-update" :label="c('app.newer', { v: latest!.latest })" label-key="IconWhatsNew" :href="latest?.releaseUrl || REPO + '/releases'" target="_blank" rel="noopener" />
      <Button v-else id="a-update" :label="update === 'ok' ? c('info.current') : c('app.check')" :label-key="update === 'ok' ? 'IconCheck' : 'IconSearch'" :tone="update === 'ok' ? 'ok' : 'neutral'" :disabled="update === 'busy'" :title="update === 'ok' ? c('app.check') : undefined" @click="check" />
    </div>
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
