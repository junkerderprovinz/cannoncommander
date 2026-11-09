<script setup lang="ts">
// A new remote starts with the product, picked by name and logo, and goes on to its fields in the same window, so
// nobody has to know that Nextcloud is WebDAV underneath. The fields come from rclone's own description of the
// backend: what it needs first, everything else folded below. Google, Microsoft and the other clouds that sign in
// through a browser get the token from rclone authorize on a computer with one, pasted here.
import { BrandMark, Button, CopyField, EmptyState, FieldRow, FieldRowGroup, PathField, Pill, ReadmeButton, SubCard, TextField } from '@glimstone/vue';
import { Select } from '@glimstone/vue/Select';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch, watchEffect } from 'vue';
import { admin, type Provider, type Remote, type RemoteCatalog, type RemoteInput, type RemoteTest, type SetupQuestion } from '@/api/admin';
import { ApiError } from '@/api/client';
import { fmtSize } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type FootButton, type WinEntry } from '@/core/windows';
import { c, fail, keysFirst, stepFoot } from '@/features/config/shared';
import { pathField } from '@/ui/picker';
import { copyWords, shake } from '@/ui/kit';
import { GROUPS, loadCatalog, markOf, nameAndKind, optionLabel, providerHint, providerName, questionLabel, splitOptions, suggestName } from '../remotes';
import RemoteField from './RemoteField.vue';

const props = defineProps<{ win: WinEntry; remote: Remote | null; all: Remote[]; saved: () => void }>();
const r = props.remote, isNew = !r;
const taken = props.all.filter((x) => x !== r).map((x) => x.name);

const cat = shallowRef<RemoteCatalog | null>(null);
const broken = ref('');
const provider = shallowRef<Provider | null>(null);
const typeId = ref(r?.type ?? '');
const type = computed(() => cat.value?.types.find((t) => t.id === typeId.value) ?? null);

const values = reactive<Record<string, string>>({ ...(r?.params ?? {}) });
const name = ref(r?.name ?? ''), mount = ref(r?.mount ?? '/remotes/'), path = ref(r?.path ?? '');
let mountTouched = !!r, nameTouched = !!r;
const token = ref(''), signing = ref(false), pasted = ref('');
const signedIn = computed(() => !!token.value || !!r?.secrets?.includes('token'));

const query = ref('');
const floor = ref(0), tiles = ref<HTMLElement | null>(null);
const more = ref(false);
const testing = ref(false);
const result = ref<RemoteTest | null>(null);
// What a test brought that outlives the next change: the question to answer and the shares to pick from.
const question = ref<SetupQuestion | null>(null);
const shares = ref<string[]>([]);
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;

watch([values, path, token], () => { result.value = null; });

const who = computed(() => (provider.value ? providerName(provider.value) : type.value?.description ?? typeId.value));
const split = computed(() => (type.value ? splitOptions(type.value, provider.value?.preset, values) : { first: [], rest: [] }));
const pathWord = computed(() => type.value?.pathHint ?? 'folder');

// Products in the reader's alphabet, found by the name shown and by rclone's own, which is what a guide online uses.
const sorted = computed(() => [...(cat.value?.providers ?? [])].sort((a, b) => providerName(a).localeCompare(providerName(b), undefined, { sensitivity: 'base' })));
const groups = computed(() => {
  const q = query.value.trim().toLowerCase();
  const hit = (words: string[]) => !q || words.some((w) => w.toLowerCase().includes(q));
  return GROUPS.map((g) => ({
    g,
    providers: sorted.value.filter((p) => p.group === g && hit([providerName(p), p.name, p.backend])),
    unlisted: g === 'protocol' ? (cat.value?.unlisted ?? []).filter((b) => hit([b])) : [],
  })).filter((x) => x.providers.length + x.unlisted.length > 0);
});
const tileName = (id: string, fallback: string) => {
  const p = cat.value?.providers.find((x) => x.id === id);
  return p ? nameAndKind(providerName(p)).name : fallback;
};
const tileStyle = (p: Provider) => {
  const lit = markOf(p.mark).tile;
  return lit ? { '--provider-tile': lit.tile, '--provider-ink': lit.ink } : undefined;
};

function choose(p: Provider | null, backend: string) {
  provider.value = p;
  typeId.value = backend;
  for (const k of Object.keys(values)) delete values[k];
  Object.assign(values, p?.preset ?? {});
  if (!nameTouched) {
    name.value = suggestName(p?.name ?? backend, taken);
    if (!mountTouched) mount.value = '/remotes/' + name.value;
  }
  token.value = '';
  question.value = null;
  shares.value = [];
  more.value = false;
  nextTick(() => (props.win.el?.querySelector('.a-rform input') as HTMLInputElement | null)?.focus());
}
function back() {
  typeId.value = '';
  provider.value = null;
  nextTick(() => (at('#a-rq') as HTMLInputElement | null)?.focus());
}

const fieldOf = (k: string) => at(`[data-rf="${k}"]`);
function missing(): [string, Element | null] | null {
  for (const o of split.value.first) {
    if (!o.required || values[o.name]?.trim() || o.default || (o.secret && r?.secrets?.includes(o.name))) continue;
    return [c('remotes.missing', { field: optionLabel(o.name) }), fieldOf(o.name)];
  }
  if (type.value?.oauth && !signedIn.value) return [c('remotes.signInFirst', { who: who.value }), at('#a-oauth')];
  return null;
}

// The form as rclone takes it. An empty field clears its parameter, except a secret, which stays as saved, and the
// host key the last test showed holds the remote to that server.
function input(): RemoteInput {
  const params: Record<string, string> = {};
  for (const [k, v] of Object.entries(values)) {
    if (v.trim() || r?.params?.[k] !== undefined) params[k] = v.trim();
  }
  if (token.value) params.token = token.value;
  const key = result.value?.hostKey;
  if (key && !params.host_keys) params.host_keys = key.pin;
  return { name: name.value.trim(), type: typeId.value, path: path.value.trim(), mount: mount.value.trim(), params };
}

async function test() {
  const m = missing();
  if (m) return fail(...m);
  testing.value = true;
  result.value = null;
  let got: RemoteTest;
  try {
    got = await admin.testRemote({ ...input(), ...(r ? { name: r.name } : {}) });
  } catch (e) {
    got = { ok: false, entries: 0, error: e instanceof Error ? e.message : String(e) };
  }
  testing.value = false;
  if (got.question) {
    question.value = got.question;
    await nextTick();
    shake(fieldOf(got.question.name));
    return;
  }
  result.value = got;
  if (got.shares) shares.value = got.shares;
  if (!got.ok) shake(at('#a-rtest'));
}

// The verdict at the start of the foot, beside the button that carries it.
const verdict = computed(() => {
  const x = result.value;
  if (!x) return '';
  if (x.ok) return x.free != null ? c('remotes.okFree', { free: fmtSize(x.free) }) : c('remotes.ok');
  if (x.failure === 'missing') return c('remotes.fail.missing', { path: path.value.trim() });
  if (x.failure) return c(`remotes.fail.${x.failure}`);
  return c('remotes.fail.other', { error: x.error ?? '' });
});

async function useToken() {
  if (!pasted.value.trim()) return fail(c('remotes.needToken'), at('#a-rtoken'));
  const got = await admin.remoteToken(typeId.value, pasted.value.trim()).catch(() => null);
  if (!got?.token) return fail(c('remotes.tokenRejected'), at('#a-rtoken'));
  token.value = got.token;
  signing.value = false;
  pasted.value = '';
  toast(c('remotes.signedIn', { who: who.value }));
}

async function save() {
  const n = name.value.trim(), m = mount.value.trim();
  if (!/^[a-z0-9][a-z0-9_-]*$/.test(n)) return fail(n ? c('lowercaseOnly') : c('remotes.needName'), at('#a-rname'));
  if (taken.includes(n)) return fail(c('nameTaken'), at('#a-rname'));
  if (!/^\/remotes\/.+/.test(m)) return fail(c('remotes.underRemotes', { name: n }), at('#a-rmount'));
  const miss = missing();
  if (miss) return fail(...miss);
  let out: Remote;
  try {
    out = r ? await admin.updateRemote(r.name, input()) : await admin.addRemote(input());
  } catch (e) {
    // rclone's setup has a choice to make first, which the test brings up.
    if (e instanceof ApiError && e.problem?.code === 'setupQuestion') return void test();
    return fail(c('notSaved'), at('#a-rname'));
  }
  props.win.close();
  toast(out.error ? c('remotes.savedNotMounted', { name: n }) : isNew ? c('remotes.addedAt', { name: n, mount: m }) : c('saved'), out.error ? 'warn' : 'ok');
  props.saved();
}

watchEffect(() => {
  if (!typeId.value) return stepFoot(props.win, [cancelBtn()]);
  const x = result.value;
  const tested: FootButton = {
    label: testing.value ? c('checking') : !x ? c('test') : x.ok ? c('remotes.connected') : c('remotes.notConnected'),
    g: testing.value ? 'IconRefresh' : !x ? 'IconHealth' : x.ok ? 'IconCheck' : 'IconClose',
    tone: x ? (x.ok ? 'ok' : 'danger') : undefined,
    disabled: testing.value,
    attrs: { id: 'a-rtest' },
    run: () => { void test(); return false; },
  };
  stepFoot(props.win, [
    { start: verdict.value },
    ...(isNew ? [{ label: c('back'), g: 'IconBack', run: () => { back(); return false; } }] : []),
    cancelBtn(),
    tested,
    { label: isNew ? c('add') : c('save'), g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: () => { void save(); return false; } },
  ]);
});

function nameTyped() {
  nameTouched = true;
  if (!mountTouched) mount.value = '/remotes/' + name.value.trim().toLowerCase();
}

let stopKeys = () => {};
onMounted(() => {
  loadCatalog().then(async (got) => {
    cat.value = got;
    if (r) provider.value = got.providers.find((p) => p.id === r.provider) ?? null;
    await nextTick();
    // The window keeps the height it opened with while a search narrows the tiles.
    floor.value = tiles.value?.offsetHeight ?? 0;
    (at('#a-rq') as HTMLInputElement | null)?.focus();
  }, (e: unknown) => { broken.value = e instanceof Error ? e.message : String(e); });
  // Escape in the search empties it first and closes the window only when it is empty.
  stopKeys = keysFirst((e) => {
    if (e.key !== 'Escape' || !query.value || document.activeElement?.id !== 'a-rq') return;
    e.preventDefault();
    e.stopPropagation();
    query.value = '';
  });
});
onBeforeUnmount(() => stopKeys());
</script>

<template>
  <EmptyState v-if="broken" nested :glyph="glyphComponent('IconCloud')" :title="c('remotes.noCatalog')" :hint="broken" />
  <EmptyState v-else-if="!cat" nested :title="c('remotes.loading')" />
  <template v-else-if="!typeId">
    <div class="a-pickhead"><TextField id="a-rq" v-model="query" :placeholder="c('remotes.search')" :aria-label="c('remotes.search')" width="medium" /></div>
    <div ref="tiles" class="a-picks" :style="floor ? { minHeight: floor + 'px' } : undefined">
      <section v-for="grp in groups" :key="grp.g" class="a-pgroup" :data-group="grp.g">
        <h3 class="glim-eyebrow">{{ c(`remotes.g.${grp.g}`) }}</h3>
        <ul class="glim-provider-pick">
          <li v-for="p in grp.providers" :key="p.id" :data-type="p.id" :title="p.group === 'protocol' ? undefined : providerHint(p.id)" :style="tileStyle(p)">
            <ReadmeButton brand="house" :parts="[{ ...nameAndKind(providerName(p)), onClick: () => choose(p, p.backend) }]" :hint="p.group === 'protocol' ? providerHint(p.id) : undefined"
              :mark-class="markOf(p.mark).svg ? undefined : 'a-pglyph'">
              <template #mark><BrandMark v-if="markOf(p.mark).svg" :svg="markOf(p.mark).svg!" /><G v-else :n="markOf(p.mark).glyph" /></template>
            </ReadmeButton>
          </li>
        </ul>
        <template v-if="grp.unlisted.length">
          <h4 class="a-pmore">{{ c('remotes.pickMore', { count: grp.unlisted.length }) }}</h4>
          <ul class="a-ppills">
            <li v-for="b in grp.unlisted" :key="b"><button type="button" class="a-ppill" :data-type="b" @click="choose(null, b)">{{ b }}</button></li>
          </ul>
        </template>
      </section>
      <p v-if="!groups.length" class="a-pnone">{{ c('remotes.searchNone', { query: query.trim(), s3: tileName('s3', 'S3'), webdav: tileName('webdav', 'WebDAV') }) }}</p>
    </div>
  </template>
  <EmptyState v-else-if="!type" nested :title="c('remotes.unknownType', { type: typeId })" />
  <div v-else class="a-rform">
    <SubCard :title="who" :hint="provider ? providerHint(provider.id) : undefined">
      <FieldRow v-if="type.oauth" :label="c('remotes.account')" :hint="c('remotes.accountTip')">
        <template v-if="signedIn"><Pill tone="ok">{{ c('remotes.signedInPill') }}</Pill><Button id="a-oauth" :label="c('remotes.otherAccount')" label-key="IconSignIn" @click="signing = true" /></template>
        <Button v-else id="a-oauth" :label="c('remotes.signInAt', { who })" label-key="IconSignIn" @click="signing = true" />
      </FieldRow>
      <FieldRowGroup v-if="type.oauth && signing">
        <FieldRow :label="c('remotes.onYourPc')" :hint="c('remotes.onYourPcTip')"><CopyField width="grow" :value="type.authorize ?? `rclone authorize &quot;${type.id}&quot;`" :label="c('remotes.onYourPc')" :text="copyWords()" /></FieldRow>
        <FieldRow :label="c('remotes.token')"><TextField id="a-rtoken" v-model="pasted" :placeholder="c('remotes.tokenPh')" spellcheck="false" autocomplete="off" width="wide" mono @keydown.enter="useToken" /><Button :label="c('remotes.takeToken')" label-key="IconCheck" @click="useToken" /></FieldRow>
      </FieldRowGroup>
      <RemoteField v-for="o in split.first" :key="o.name" :o="o" :values="values" :provider="provider" :type="type.id" :saved="r?.secrets?.includes(o.name) ?? false" />
      <FieldRow :label="c(`remotes.path.${pathWord}`)" :hint="c(`remotes.pathTip.${pathWord}`)">
        <template #label>{{ c(`remotes.path.${pathWord}`) }}{{ ' ' }}<small>{{ c('optional') }}</small></template>
        <Select v-if="shares.length" v-model="path" :options="shares.map((s) => ({ value: s, label: s }))" :placeholder="c('remotes.pickShare')" :trigger-attrs="{ 'data-rf': 'path' }" />
        <TextField v-else v-model="path" width="medium" data-rf="path" :placeholder="pathWord === 'folder' ? '/' : ''" spellcheck="false" mono />
      </FieldRow>
      <FieldRow v-if="question" :label="questionLabel(question.name)" :hint="c('remotes.q.hint')">
        <Select v-model="values[question.name]" :options="question.choices.map((x) => ({ value: x.value, label: x.help || x.value }))" :placeholder="c('remotes.q.pick')" :trigger-attrs="{ 'data-rf': question.name }" />
      </FieldRow>
      <FieldRow v-if="result?.hostKey" :label="c('remotes.hostKey')" :hint="c('remotes.hostKeyTip')">
        <span class="a-hostkey mono" data-hostkey>{{ result.hostKey.fingerprint }}</span>
      </FieldRow>
    </SubCard>
    <SubCard v-if="split.rest.length" :title="c('remotes.more')">
      <FieldRow :label="c('remotes.moreCount', { count: split.rest.length })" :hint="c('remotes.moreTip')">
        <Button id="a-rmore" :label="more ? c('remotes.moreHide') : c('remotes.moreShow')" :label-key="more ? 'IconCollapse' : 'IconExpand'" @click="more = !more" />
      </FieldRow>
      <template v-if="more">
        <RemoteField v-for="o in split.rest" :key="o.name" :o="o" :values="values" :provider="provider" :type="type.id" :saved="r?.secrets?.includes(o.name) ?? false" />
      </template>
    </SubCard>
    <SubCard>
      <FieldRowGroup>
        <FieldRow :label="c('remotes.name')" :hint="c('remotes.nameTip')"><TextField id="a-rname" v-model="name" :placeholder="c('remotes.namePh')" width="medium" @input="nameTyped" /></FieldRow>
        <FieldRow :label="c('remotes.mountAt')" :hint="c('remotes.mountAtTip')"><PathField v-model="mount" v-bind="pathField('a-rmount')" @input="mountTouched = true" /></FieldRow>
      </FieldRowGroup>
    </SubCard>
  </div>
</template>
