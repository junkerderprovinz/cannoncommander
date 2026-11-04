<script setup lang="ts">
// The office server is a choice: none uses the built-in previews, OnlyOffice and Euro-Office share one connection,
// Collabora speaks WOPI. ArrowLoop takes folder pairs as recurring jobs and sees the shares under paths of its own.
import { computed, onMounted, reactive, ref } from 'vue';
import { admin, type AppChange, type AppSettings, type IntegrationResult, type OfficeKind, type System, type Tool } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import BareBtn from '../kit/BareBtn.vue';
import Btn from '../kit/Btn.vue';
import Card from '../kit/Card.vue';
import CopyField from '../kit/CopyField.vue';
import Dropdown from '../kit/Dropdown.vue';
import Frow from '../kit/Frow.vue';
import { c, fail, shake } from '../kit/kit';
import PwField from '../kit/PwField.vue';
import Sw from '../kit/Sw.vue';

type Test = 'idle' | 'busy' | 'ok' | 'bad';
const app = ref<AppSettings | null>(null), sys = ref<System | null>(null);
const secret = ref(''), alToken = ref('');
const paths = ref<{ here: string; there: string }[]>([]);
const tests = reactive<Record<'office' | 'arrowloop', Test>>({ office: 'idle', arrowloop: 'idle' });
const officeBtn = ref<HTMLButtonElement | null>(null), alBtn = ref<HTMLButtonElement | null>(null);
const pathsEl = ref<HTMLElement | null>(null);

function take(a: AppSettings) {
  app.value = a;
  paths.value = Object.entries(a.arrowloop.paths ?? {}).map(([here, there]) => ({ here, there }));
}
onMounted(() => {
  admin.app().then(take, () => {});
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

const flag = (k: 'thumbs' | 'video') => computed({ get: () => !!app.value?.previews[k], set: (v: boolean) => { app.value!.previews[k] = v; save({ previews: { [k]: v } }); } });
const thumbs = flag('thumbs'), video = flag('video');
const webdav = computed({ get: () => !!app.value?.webdav, set: (v: boolean) => { app.value!.webdav = v; save({ webdav: v }, c('integrations.afterRestart')); } });
const webdavUrl = new URL('dav/', location.origin + import.meta.env.BASE_URL).href;
const tool = (t?: Tool) => (t?.available ? [t.version, t.path].filter(Boolean).join(', ') : c('integrations.notFound'));

const alUrl = () => app.value!.arrowloop.url.trim();
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
    if (!here.startsWith('/') || !there.startsWith('/')) return fail(c('integrations.fullPaths'), pathsEl.value?.querySelectorAll('.frow')[i]);
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
  const btn = what === 'office' ? officeBtn.value : alBtn.value;
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
const testGlyph = (s: Test) => (s === 'ok' ? 'IconCheck' : s === 'bad' ? 'IconClose' : s === 'busy' ? 'IconRefresh' : 'IconHealth');
const testWord = (s: Test) => (s === 'ok' ? c('remotes.connected') : s === 'bad' ? c('integrations.rejected') : s === 'busy' ? c('checking') : c('test'));
</script>

<template>
  <template v-if="app && office">
    <Card :title="c('integrations.office')">
      <Frow :label="c('integrations.officeKind')" :tip="c('integrations.officeKindTip')">
        <Dropdown v-model="kind" :options="[['none', c('integrations.none')], ['onlyoffice', 'OnlyOffice'], ['eurooffice', 'Euro-Office'], ['collabora', 'Collabora Online']]" />
      </Frow>
      <template v-if="kind !== 'none'">
        <Frow :label="c('integrations.url')" :tip="c('integrations.urlTip')">
          <span class="a-ctl"><input id="a-oourl" v-model="office.url" class="field w" placeholder="https://office.tower.lan" @input="tests.office = 'idle'" @change="saveOffice">
            <button id="a-ootest" ref="officeBtn" class="btn" :class="{ ok: tests.office === 'ok', bad: tests.office === 'bad' }" :disabled="tests.office === 'busy'" @click="test('office')"><G :n="testGlyph(tests.office)" />{{ testWord(tests.office) }}</button></span>
        </Frow>
        <Frow v-if="needsSecret" :label="c('integrations.secret')" :tip="c('integrations.secretTip')"><PwField id="a-oosecret" v-model="secret" :placeholder="office.secretSet ? c('integrations.kept') : 'JWT_SECRET'" @change="saveSecret" /></Frow>
      </template>
    </Card>
    <Card :title="c('integrations.media')">
      <Frow :label="c('integrations.thumbs')" :tip="c('integrations.thumbsTip')"><Sw v-model="thumbs" /></Frow>
      <Frow :label="c('integrations.video')" :tip="c('integrations.videoTip')"><Sw v-model="video" /></Frow>
      <Frow label="ffmpeg" :tip="c('integrations.ffmpegTip')"><span class="pill mono">{{ sys ? tool(sys.tools.ffmpeg) : '' }}</span></Frow>
    </Card>
    <Card title="ArrowLoop" :hint="c('integrations.alHint')">
      <Frow :label="c('integrations.address')" :tip="c('integrations.alUrlTip')">
        <span class="a-ctl"><input id="a-alurl" v-model="app.arrowloop.url" class="field w" placeholder="http://tower.lan:8095" @input="tests.arrowloop = 'idle'" @change="saveAlUrl">
          <button id="a-altest" ref="alBtn" class="btn" :class="{ ok: tests.arrowloop === 'ok', bad: tests.arrowloop === 'bad' }" :disabled="tests.arrowloop === 'busy'" @click="test('arrowloop')"><G :n="testGlyph(tests.arrowloop)" />{{ testWord(tests.arrowloop) }}</button></span>
      </Frow>
      <Frow :label="c('integrations.alToken')" :tip="c('integrations.alTokenTip')"><PwField id="a-altoken" v-model="alToken" :placeholder="app.arrowloop.tokenSet ? c('integrations.kept') : ''" @change="saveAlToken" /></Frow>
      <div ref="pathsEl">
        <Frow v-if="!paths.length" :label="c('integrations.alPaths')" :tip="c('integrations.alPathsTip')"><span class="pill">{{ c('integrations.samePaths') }}</span></Frow>
        <Frow v-for="(p, i) in paths" :key="i" :label="i ? '' : c('integrations.alPaths')" :tip="i ? undefined : c('integrations.alPathsTip')">
          <span class="a-ctl"><input v-model="p.here" class="field m mono" placeholder="/mnt/user" :aria-label="c('integrations.here')" @change="savePaths"><G n="IconForward" class="a-to" /><input v-model="p.there" class="field m mono" placeholder="/data" :aria-label="c('integrations.there')" @change="savePaths"><BareBtn g="IconTrash" :tip="c('remove')" @click="removePath(i)" /></span>
        </Frow>
      </div>
      <div class="a-foot"><Btn :label="c('integrations.addPath')" g="IconAdd" @click="paths.push({ here: '', there: '' })" /></div>
    </Card>
    <Card :title="c('integrations.webdavRclone')">
      <Frow :label="c('integrations.webdav')" :tip="c('integrations.webdavTip')"><Sw v-model="webdav" /></Frow>
      <Frow v-if="webdav" :label="c('integrations.address')"><CopyField :value="webdavUrl" /></Frow>
      <Frow label="rclone" :tip="c('integrations.rcloneTip')"><span class="pill mono">{{ sys ? tool(sys.tools.rclone) : '' }}</span></Frow>
    </Card>
  </template>
</template>
