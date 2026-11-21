<script setup lang="ts">
import { Card, InfoComponentsCard, InfoVersionCard } from '@glimstone/vue';
import { componentText, type ComponentRow, type UpdateRoute, type UpdateState } from '@glimstone/core';
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { admin, type AppSettings, type Modules, type SelfUpdate, type System, type Tool, type UpdateCheck } from '@/api/admin';
import { toast } from '@/core/toast';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { win } from '@/core/windows';
import { language } from '@/i18n';
import { unseen } from '../help';
import { c } from '@/features/config/shared';
import { bugReport, DISCORD, DOCS, REPO } from '../support';
import CoffeeWin from '../wins/CoffeeWin.vue';
import CryptoWin from '../wins/CryptoWin.vue';
import PaypalWin from '../wins/PaypalWin.vue';

const sys = ref<System | null>(null), app = ref<AppSettings | null>(null);
const words = computed(() => componentText(language()));
onMounted(() => {
  admin.system().then((s) => { sys.value = s; }, () => {});
  admin.app().then((a) => { app.value = a; }, () => {});
  // The container answers 404: its helper programs come with the image.
  admin.modules().then((m) => { mods.value = m; if (m.running) follow(); }, () => {});
});
const saveUpdates = (change: Partial<AppSettings['updates']>) => {
  Object.assign(app.value!.updates, change);
  admin.saveApp({ updates: change }).then(() => toast(c('saved')), () => toast(c('notSaved'), 'warn'));
};

// The desktop app installs its own updates, a Flatpak leaves them to the software centre, and a container is updated
// by its container manager.
const route = computed<UpdateRoute>(() => (sys.value?.deployment !== 'desktop' ? 'guide' : sys.value.flatpak ? 'store' : 'install'));
// One switch, the one this kind of copy has: installing by itself, or only looking.
const autoUpdate = computed({
  get: () => (!app.value ? undefined : route.value === 'install' ? app.value.updates.autoUpdate : route.value === 'guide' ? app.value.updates.autoCheck : undefined),
  set: (v: boolean | undefined) => saveUpdates(route.value === 'install' ? { autoUpdate: !!v } : { autoCheck: !!v }),
});

const plain = (v?: string) => v?.replace(/^v/, '') ?? '';
const latest = ref<UpdateCheck | null>(null);
const checked = ref<UpdateState>({ kind: 'current' });
const own = ref<SelfUpdate | null>(null);

// A check nobody pressed for stays quiet when GitHub does not answer.
async function check(quiet = false) {
  checked.value = { kind: 'checking' };
  if (own.value) own.value = { ...own.value, failed: undefined };
  const got = await admin.checkUpdate().catch(() => null);
  latest.value = got;
  if (!got) checked.value = quiet ? { kind: 'current' } : { kind: 'failed', reason: c('app.noAnswer') };
  else checked.value = got.updateAvailable ? { kind: 'available', version: plain(got.latest) } : { kind: 'current', checkedAt: new Date() };
}
// Once as the page opens while the switch is on, and once more when it is turned on.
watch(autoUpdate, (on) => { if (on) void check(true); });

// The desktop app's own update, followed every second while it downloads.
let ownPoll: number | undefined;
function followOwn() {
  window.clearTimeout(ownPoll);
  ownPoll = window.setTimeout(async () => {
    const got = await admin.selfUpdate().catch(() => null);
    if (got) own.value = got;
    if (got?.busy) followOwn();
  }, 1000);
}
watch(route, (r) => {
  if (r === 'install') admin.selfUpdate().then((s) => { own.value = s; if (s.busy) followOwn(); }, () => {});
});
async function updateNow() {
  const got = await admin.updateNow().catch(() => null);
  if (!got) { toast(c('app.noAnswer'), 'warn'); return; }
  own.value = got;
  followOwn();
}

const state = computed<UpdateState>(() => {
  if (route.value === 'store') return { kind: 'store', store: 'Flathub' };
  const ready = own.value?.ready || latest.value?.ready;
  if (ready) return { kind: 'ready', version: plain(ready) };
  if (own.value?.busy) return own.value.percent >= 0 ? { kind: 'downloading', percent: own.value.percent } : { kind: 'checking' };
  if (own.value?.failed) return { kind: 'failed', reason: own.value.failed };
  return checked.value;
});
const open = (url: string) => window.open(url, '_blank', 'noopener');
// A new version's notes are not in this build, so they open on its release page.
function whatsNew() {
  const s = state.value;
  if (s.kind === 'available' || s.kind === 'ready') open(`${REPO}/releases/tag/v${s.version}`);
  else run('whatsNew');
}

// The helper programs the server runs. The desktop app fetches a missing one with the others of its version.
const mods = ref<Modules | null>(null);
const fetchable = (t?: Tool) => !!mods.value?.offered && !t?.available && !t?.checking;
let poll: number | undefined;
function follow() {
  window.clearTimeout(poll);
  poll = window.setTimeout(async () => {
    const got = await admin.modules().catch(() => null);
    if (got) mods.value = got;
    if (got?.running) follow();
    else if (got?.failed) toast(c('info.fetchFailed'), 'warn');
  }, 1000);
}
async function fetchModules() {
  const got = await admin.fetchModules().catch(() => null);
  if (!got) { toast(c('info.fetchFailed'), 'warn'); return; }
  mods.value = got;
  follow();
}
onUnmounted(() => { window.clearTimeout(poll); window.clearTimeout(ownPoll); });

// What a helper's row says beside its number, and the one thing to do about a missing one.
function helper(name: string, tool: Tool | undefined, hint: string, where: Pick<ComponentRow, 'repoUrl' | 'releasePage'>): ComponentRow {
  const row: ComponentRow = { name, kind: 'tool', hint, version: plain(tool?.version), ...(tool?.version ? where : {}) };
  if (tool?.checking) return { ...row, status: c('integrations.checking') };
  if (tool?.available) return tool.version ? row : { ...row, version: tool.path ?? '' };
  if (!fetchable(tool)) return { ...row, status: c('integrations.notFound'), tone: 'warn' };
  if (mods.value?.ready) return { ...row, action: { label: c('info.fetched'), labelKey: 'IconRefresh', onClick: () => void admin.restart() } };
  if (mods.value?.running) return { ...row, status: c('info.fetching', { n: Math.round(mods.value.fraction * 100) }) };
  // The button says what is missing; a status beside it leaves a phone no room for the name.
  return { ...row, action: { label: c('info.download'), labelKey: 'IconDownload', onClick: () => void fetchModules() } };
}
const rows = computed<ComponentRow[]>(() => {
  const tools = sys.value?.tools;
  if (!tools) return [];
  return [
    helper('rclone', tools.rclone, c('info.rcloneTip'), { repoUrl: 'https://github.com/rclone/rclone' }),
    helper('ffmpeg', tools.ffmpeg, c('info.ffmpegTip'), { releasePage: 'https://ffmpeg.org/download.html#releases' }),
    helper('7-Zip', tools['7zz'], c('info.sevenZipTip'), { repoUrl: 'https://github.com/ip7z/7zip' }),
    helper('MuPDF', tools.mutool, c('info.mupdfTip'), { releasePage: 'https://mupdf.com/releases' }),
    ...(sys.value?.installer
      ? [{ name: 'PommelPorter', kind: 'installer' as const, hint: words.value.componentsCard.installer, version: sys.value.installer, repoUrl: 'https://github.com/junkerderprovinz/pommelporter' }]
      : []),
  ];
});
const n = computed(() => unseen());

const readme = (tile: string, mark: string, glyph: string, name: string, sub: string | undefined, go: () => unknown, aria?: string, id?: string) => ({ tile, mark, glyph, name, sub, go, aria, id });
const help = computed(() => [
  readme('glim-tile-github', 'glim-github-mark', 'IconGithub', 'GitHub', undefined, () => open(REPO)),
  readme('glim-tile-discord', 'glim-discord-mark', 'IconDiscord', 'Discord', undefined, () => open(DISCORD)),
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
  <InfoVersionCard
    v-model:auto-update="autoUpdate"
    :text="words.versionCard"
    name="CannonCommander"
    :version="plain(sys?.version)"
    :repo-url="REPO"
    :route="route"
    :state="state"
    :on-check="() => check()"
    :on-update="updateNow"
    :on-restart="() => void admin.restart()"
    :guide-url="`${DOCS}getting-started/docker/#updates`"
    :on-whats-new="whatsNew"
    :unseen="n"
    :open-url="open"
  />
  <InfoComponentsCard :text="words.componentsCard" name="CannonCommander" :rows="rows" :open-url="open" />
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
