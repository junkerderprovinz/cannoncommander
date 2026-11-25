<script setup lang="ts">
// The office server is a choice: none uses the built-in previews, OnlyOffice and Euro-Office share one connection,
// Collabora speaks WOPI. Servers running next to this one are offered for one click. ArrowLoop takes folder pairs as recurring jobs and sees the shares under paths of its own.
import { Select } from '@glimstone/vue/Select';
import { Button, Card, ConnectionRow, CopyField, FieldRow, FieldRowGroup, IconButton, PasswordField, Pill, TextField, Toggle, type ConnectionAction } from '@glimstone/vue';
import { computed, nextTick, onMounted, reactive, ref } from 'vue';
import { lazyComponent } from '@/core/lazy';
import { admin, type AppChange, type AppSettings, type FoundOffice, type IntegrationResult, type OfficeKind, type System, type Tool } from '@/api/admin';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { OFFICE_MARKS } from '@/features/config/officeMarks';
import { c, fail } from '@/features/config/shared';
import { terminalOn } from '@/features/terminal/state';
import { copyWords, eyeWords, selectOptions, shake } from '@/ui/kit';

type Test = 'idle' | 'busy' | 'ok' | 'bad';
// The card carries the marks of thirty clients, which only this tab needs.
const McpCard = lazyComponent(() => import('../mcp/McpCard.vue'));
const app = ref<AppSettings | null>(null), sys = ref<System | null>(null);
const secret = ref(''), alToken = ref('');
const paths = ref<{ here: string; there: string }[]>([]);
const tests = reactive<Record<'office' | 'arrowloop', Test>>({ office: 'idle', arrowloop: 'idle' });
const officeBtn = ref<InstanceType<typeof Button> | null>(null), alBtn = ref<InstanceType<typeof Button> | null>(null);
const pathsEl = ref<HTMLElement | null>(null);

function take(a: AppSettings) {
  app.value = a;
  paths.value = Object.entries(a.arrowloop.paths ?? {}).map(([here, there]) => ({ here, there }));
}
onMounted(() => {
  admin.app().then(take, () => {});
  admin.foundOffices().then((f) => { offices.value = f; }, () => {});
  admin.system().then((s) => { sys.value = s; }, () => {});
});

async function save(change: AppChange, note = c('saved')) {
  try {
    const got = await admin.saveApp(change);
    if (app.value) { app.value.office.secretSet = got.office.secretSet; app.value.arrowloop.tokenSet = got.arrowloop.tokenSet; }
    toast(note);
    return true;
  } catch {
    toast(c('notSaved'), 'warn');
    return false;
  }
}

const office = computed(() => app.value?.office);
const needsSecret = computed(() => office.value?.kind === 'onlyoffice' || office.value?.kind === 'eurooffice');
// A server needs its address before it can be saved, so picking one waits for the address.
const kind = computed({
  get: () => office.value?.kind ?? 'none',
  set: (k: OfficeKind) => {
    app.value!.office.kind = k;
    tests.office = 'idle';
    if (k === 'none' || office.value!.url.trim()) save({ office: { kind: k, url: office.value!.url.trim() } });
  },
});
const saveOffice = () => { if (office.value!.url.trim()) save({ office: { kind: office.value!.kind, url: office.value!.url.trim() } }); };
async function saveSecret() {
  if (!secret.value) return;
  if (await save({ office: { secret: secret.value } })) secret.value = '';
}

// Without Docker the page asks the host the browser came from, which is where the office server usually runs too.
const offices = ref<{ docker: boolean; servers: FoundOffice[] } | null>(null);
const host = ref(location.hostname);
const looking = ref(false), joining = ref('');
const lookBtn = ref<InstanceType<typeof Button> | null>(null);
const suites: Record<FoundOffice['kind'], string> = { onlyoffice: 'OnlyOffice', eurooffice: 'Euro-Office', collabora: 'Collabora Online' };
const rowText = computed(() => ({ test: c('test'), edit: c('edit'), remove: c('remove') }));
const keyOf = (s: FoundOffice) => s.id ?? s.url;
const isOurs = (s: FoundOffice) => office.value?.kind === s.kind && office.value?.url === s.url;
const foundActions = (s: FoundOffice): ConnectionAction[] => (isOurs(s) ? [] : [{
  id: 'connect', label: c('integrations.connect'), labelKey: 'IconLink', busy: joining.value === keyOf(s), onClick: () => { void join(s); },
}]);

async function look() {
  const h = host.value.trim();
  if (!h) return fail(c('integrations.lookHost'), lookBtn.value?.el);
  looking.value = true;
  const got = await admin.probeOffices(h).catch(() => null);
  looking.value = false;
  if (!got) return fail(c('integrations.hostOnly'), lookBtn.value?.el);
  offices.value = { docker: false, servers: got.servers };
  if (!got.servers.length) toast(c('integrations.lookNone', { host: h }));
}

async function askSecret() {
  toast(c('integrations.enterSecret'));
  await nextTick();
  document.querySelector<HTMLInputElement>('#a-oosecret')?.focus();
}

// A container's secret travels from Docker into the settings on the server. A probe only knows the address, so the
// secret is typed in afterwards.
async function join(s: FoundOffice) {
  const o = app.value!.office, name = suites[s.kind];
  joining.value = keyOf(s);
  try {
    if (s.id) {
      const got = await admin.connectOffice(s.id);
      Object.assign(o, got.office);
      tests.office = got.result.ok ? 'ok' : 'bad';
      if (got.result.ok) toast(c('integrations.connectedTo', { name }));
      else if (got.result.reachable && got.result.token === 'missing') await askSecret();
      else toast(why(got.result, 'office'), 'warn');
      return;
    }
    o.kind = s.kind;
    o.url = s.url;
    tests.office = 'idle';
    if (!(await save({ office: { kind: s.kind, url: s.url } }, c('integrations.connectedTo', { name })))) return;
    if (needsSecret.value && !o.secretSet) await askSecret();
  } catch {
    toast(c('notSaved'), 'warn');
  } finally {
    joining.value = '';
  }
}

const flag = (k: 'thumbs' | 'video') => computed({ get: () => !!app.value?.previews[k], set: (v: boolean) => { app.value!.previews[k] = v; save({ previews: { [k]: v } }); } });
const thumbs = flag('thumbs'), video = flag('video');
const webdav = computed({ get: () => !!app.value?.webdav, set: (v: boolean) => { app.value!.webdav = v; save({ webdav: v }); } });
// The terminal's commands follow the switch at once, without a fresh look at the server.
const terminal = computed({ get: () => !!app.value?.terminal, set: (v: boolean) => {
  app.value!.terminal = v;
  void save({ terminal: v }).then((ok) => { if (ok) terminalOn.value = v; });
} });
const webdavUrl = new URL('dav/', location.origin + import.meta.env.BASE_URL).href;
const desktop = computed(() => sys.value?.deployment === 'desktop');
const tool = (t?: Tool) => (t?.checking ? c('integrations.checking') : t?.available ? [t.version, t.path].filter(Boolean).join(', ') : c('integrations.notFound'));

const alAddress = computed({ get: () => app.value?.arrowloop.url ?? '', set: (url) => { app.value!.arrowloop.url = url; } });
const alTest = computed(() => tests.arrowloop);
const alTouched = () => { tests.arrowloop = 'idle'; };
const alUrl = () => alAddress.value.trim();
const saveAlUrl = () => save({ arrowloop: { url: alUrl() } });
async function saveAlToken() {
  if (!alToken.value) return;
  if (await save({ arrowloop: { token: alToken.value } })) alToken.value = '';
}
// A pair counts once both sides are filled in, so a row being typed is not saved half done.
function savePaths() {
  const out: Record<string, string> = {};
  for (const [i, p] of paths.value.entries()) {
    const here = p.here.trim().replace(/\/+$/, ''), there = p.there.trim().replace(/\/+$/, '');
    if (!here && !there) continue;
    if (!here || !there) return;
    if (!here.startsWith('/') || !there.startsWith('/')) return fail(c('integrations.fullPaths'), pathsEl.value?.querySelectorAll('.glim-field-row')[i]);
    out[here] = there;
  }
  save({ arrowloop: { paths: out } });
}
function removePath(i: number) {
  paths.value.splice(i, 1);
  savePaths();
}

const why = (r: IntegrationResult, what: 'office' | 'arrowloop') => {
  if (!r.reachable) return what === 'office' ? c('integrations.unreachable') : c('integrations.alUnreachable');
  if (r.token === 'missing') return what === 'office' ? c('integrations.wantsSecret') : c('integrations.alWantsToken');
  if (r.token === 'rejected') return what === 'office' ? c('integrations.secretWrong') : c('integrations.alTokenWrong');
  return c('integrations.notWorking');
};
async function test(what: 'office' | 'arrowloop') {
  const url = what === 'office' ? office.value!.url.trim() : alUrl();
  const btn = (what === 'office' ? officeBtn.value : alBtn.value)?.el;
  if (!url) return fail(c('integrations.needUrl'), btn);
  if (!/^https?:\/\/[^/\s]+/.test(url)) return fail(c('integrations.badUrl'), btn);
  tests[what] = 'busy';
  const k = what === 'office' ? office.value!.kind as Exclude<OfficeKind, 'none'> : 'arrowloop';
  const got = await admin.testIntegration(k, url, what === 'office' ? secret.value : alToken.value).catch(() => null);
  tests[what] = got?.ok ? 'ok' : 'bad';
  if (got?.ok) return;
  shake(btn);
  toast(got ? why(got, what) : c('notSaved'), 'warn');
}
const testTone = (s: Test) => (s === 'ok' ? 'ok' : s === 'bad' ? 'danger' : 'neutral');
const testGlyph = (s: Test) => (s === 'bad' ? 'IconClose' : s === 'busy' ? 'IconRefresh' : 'IconCheck');
const testWord = (s: Test) => (s === 'ok' ? c('remotes.connected') : s === 'bad' ? c('integrations.rejected') : s === 'busy' ? c('checking') : c('test'));
</script>

<template>
  <template v-if="app && office">
    <Card :title="c('integrations.office')">
      <FieldRow :label="c('integrations.officeKind')" :hint="c('integrations.officeKindTip')">
        <Select v-model="kind" :options="selectOptions([['none', c('integrations.none')], ['onlyoffice', 'OnlyOffice'], ['eurooffice', 'Euro-Office'], ['collabora', 'Collabora Online']])" />
      </FieldRow>
      <template v-if="kind !== 'none'">
        <FieldRow :label="c('integrations.url')" :hint="c('integrations.urlTip')">
          <TextField id="a-oourl" v-model="office.url" placeholder="https://office.tower.lan" width="wide" @input="tests.office = 'idle'" @change="saveOffice" />
            <Button id="a-ootest" ref="officeBtn" :label="testWord(tests.office)" :label-key="testGlyph(tests.office)" :tone="testTone(tests.office)" :disabled="tests.office === 'busy'" @click="test('office')" />
        </FieldRow>
        <FieldRow v-if="needsSecret" :label="c('integrations.secret')" :hint="c('integrations.secretTip')"><PasswordField id="a-oosecret" v-model="secret" :placeholder="office.secretSet ? c('integrations.kept') : 'JWT_SECRET'" @change="saveSecret" width="wide" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
      </template>
      <div v-if="offices" data-found>
        <FieldRow v-if="!offices.docker" :label="c('integrations.look')" :hint="c('integrations.lookTip')">
          <TextField v-model="host" :aria-label="c('integrations.lookHost')" placeholder="tower.lan" width="medium" @keydown.enter="look" />
          <Button ref="lookBtn" data-probe :label="c('integrations.lookButton')" label-key="IconSearch" :busy="looking" @click="look" />
        </FieldRow>
        <FieldRow v-if="offices.docker || offices.servers.length" :label="c('integrations.found')" :hint="c('integrations.foundTip')">
          <Pill v-if="!offices.servers.length">{{ c('integrations.foundNone') }}</Pill>
        </FieldRow>
        <div v-if="offices.servers.length" class="a-conns">
          <ConnectionRow v-for="s in offices.servers" :key="keyOf(s)" :name="suites[s.kind]" :mark="OFFICE_MARKS[s.kind]" :status="s.url" :text="rowText" :actions="foundActions(s)">
            <template #badges>
              <Pill v-if="isOurs(s)" tone="ok">{{ c('remotes.connected') }}</Pill>
              <Pill v-else-if="!s.reachable" tone="warn" :title="c('integrations.noAnswerTip')">{{ c('integrations.noAnswer') }}</Pill>
            </template>
            <p v-if="s.name" class="a-conn-path mono">{{ s.name }}</p>
            <p v-if="s.wopi === 'blocked'" class="a-found-note">{{ c('integrations.wopiBlocked', { address: s.allow ?? '' }) }}</p>
            <p v-else-if="s.secret === 'unknown' && !isOurs(s)" class="a-found-note">{{ c('integrations.ownSecret') }}</p>
          </ConnectionRow>
        </div>
      </div>
    </Card>
    <Card :title="c('integrations.media')">
      <FieldRow :label="c('integrations.thumbs')" :hint="c('integrations.thumbsTip')"><Toggle v-model="thumbs" data-it="thumbs" /></FieldRow>
      <FieldRow :label="c('integrations.video')" :hint="c('integrations.videoTip')"><Toggle v-model="video" data-it="videoPreview" /></FieldRow>
      <FieldRow label="ffmpeg" :hint="c(desktop ? 'integrations.ffmpegDesktopTip' : 'integrations.ffmpegTip')"><Pill class="mono">{{ sys ? tool(sys.tools.ffmpeg) : '' }}</Pill></FieldRow>
    </Card>
    <Card title="ArrowLoop" :hint="c('integrations.alHint')">
      <FieldRow :label="c('integrations.address')" :hint="c('integrations.alUrlTip')">
        <TextField id="a-alurl" v-model="alAddress" placeholder="http://tower.lan:8095" width="wide" @input="alTouched" @change="saveAlUrl" />
          <Button id="a-altest" ref="alBtn" :label="testWord(alTest)" :label-key="testGlyph(alTest)" :tone="testTone(alTest)" :disabled="alTest === 'busy'" @click="test('arrowloop')" />
      </FieldRow>
      <FieldRow :label="c('integrations.alToken')" :hint="c('integrations.alTokenTip')"><PasswordField id="a-altoken" v-model="alToken" :placeholder="app?.arrowloop.tokenSet ? c('integrations.kept') : ''" @change="saveAlToken" width="wide" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
      <div ref="pathsEl">
        <FieldRow v-if="!paths.length" :label="c('integrations.alPaths')" :hint="c('integrations.alPathsTip')"><Pill>{{ c('integrations.samePaths') }}</Pill></FieldRow>
        <FieldRow v-for="(p, i) in paths" :key="i" :label="i ? '' : c('integrations.alPaths')" :hint="i ? undefined : c('integrations.alPathsTip')">
          <TextField v-model="p.here" placeholder="/mnt/user" :aria-label="c('integrations.here')" width="medium" mono @change="savePaths" /><G n="IconForward" class="a-to" /><TextField v-model="p.there" placeholder="/data" :aria-label="c('integrations.there')" width="medium" mono @change="savePaths" /><IconButton :glyph="glyphComponent('IconTrash')" :tip="c('remove')" @click="removePath(i)" />
        </FieldRow>
      </div>
      <div class="a-foot"><Button :label="c('integrations.addPath')" label-key="IconAdd" @click="paths.push({ here: '', there: '' })" /></div>
    </Card>
    <Card :title="c('integrations.webdavRclone')">
      <FieldRowGroup>
        <FieldRow :label="c('integrations.webdav')" :hint="c('integrations.webdavTip')"><Toggle v-model="webdav" data-it="webdav" /></FieldRow>
        <FieldRow v-if="webdav" :label="c('integrations.address')"><CopyField width="grow" :value="webdavUrl" :label="c('integrations.address')" :text="copyWords()" /></FieldRow>
        <FieldRow label="rclone" :hint="c(desktop ? 'integrations.rcloneDesktopTip' : 'integrations.rcloneTip')"><Pill class="mono">{{ sys ? tool(sys.tools.rclone) : '' }}</Pill></FieldRow>
      </FieldRowGroup>
    </Card>
    <Card :title="c('integrations.terminalCard')">
      <!-- The desktop app runs on the user's own computer and opens its terminal without a switch. -->
      <FieldRow v-if="desktop" :label="c('integrations.terminalDesktop')" :hint="c('integrations.terminalDesktopTip')" />
      <FieldRow v-else :label="c('integrations.terminal')" :hint="c('integrations.terminalTip')"><Toggle v-model="terminal" data-it="terminal" /></FieldRow>
    </Card>
  </template>
  <!-- Every account has keys of its own, so the card does not wait for the settings only admins may read. -->
  <McpCard />
</template>
