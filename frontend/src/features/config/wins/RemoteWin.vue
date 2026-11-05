<script setup lang="ts">
// A remote in two steps: first the kind, then its fields. Google, Microsoft and Dropbox sign in through
// rclone authorize on a computer with a browser, and the token it prints is pasted here.
import { pathField } from '@/ui/picker';
import { Select } from '@glimstone/vue/Select';
import { Button, CopyField, FieldRow, PasswordField, PathField, Pill, TextField } from '@glimstone/vue';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { admin, type Remote, type RemoteInput } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type FootButton, type WinEntry } from '@/core/windows';
import { attempt, c, fail, keysFirst, stepFoot } from '@/features/config/shared';
import { copyWords, eyeWords, selectOptions } from '@/ui/kit';
import { type RemoteType, remoteFields, remoteTypes, typeOf } from '../remotes';

const WIDTHS = { w: 'wide', m: 'medium', s: 'short' } as const;
const props = defineProps<{ win: WinEntry; remote: Remote | null; all: Remote[]; saved: () => void }>();
const r = props.remote, isNew = !r;
const type = ref<RemoteType | null>(r ? typeOf(r.type) : null);
const draft = reactive<Record<string, string>>({ name: r?.name ?? '', mount: r?.mount ?? '/mnt/remotes/', ...(r?.params ?? {}) });
const token = ref('');
const signedIn = computed(() => !!token.value || !!r?.secrets?.includes('token'));
const query = ref('');
const tested = ref<'no' | 'busy' | 'ok'>('no');
const signing = ref(false), pasted = ref('');
let mountTouched = !!r;
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;
// A changed field needs a new test before the button says it is connected again.
watch(draft, () => { if (tested.value === 'ok') tested.value = 'no'; });

const shown = computed(() => remoteTypes().filter((t) => (t.label + ' ' + t.sub).toLowerCase().includes(query.value.toLowerCase())));
const fields = computed(() => (type.value ? remoteFields(type.value) : []));

function missing(): [string, Element | null] | null {
  for (const f of fields.value) {
    if (f.opt || f.dd || (f.pw && !isNew)) continue;
    if (!draft[f.k]?.trim()) return [c('remotes.missing', { field: f.l }), at(`[data-rf="${f.k}"]`)];
  }
  if (type.value?.oauth && !signedIn.value) return [c('remotes.signInFirst', { who: type.value.oauth }), at('#a-oauth')];
  return null;
}

function choose(t: RemoteType) {
  type.value = t;
  defaults(t);
  tested.value = 'no';
  showForm();
}

function defaults(t: RemoteType) {
  for (const f of remoteFields(t)) {
    if (f.path && r) draft[f.k] ??= r.path;
    if (!draft[f.k]) draft[f.k] = f.def ?? f.dd?.[0]?.[0] ?? '';
  }
}

function showPick() {
  type.value = null;
  stepFoot(props.win, [cancelBtn()]);
  nextTick(() => (at('#a-rq') as HTMLInputElement | null)?.focus());
}
function showForm() {
  const foot: FootButton[] = [...(isNew ? [{ label: c('back'), g: 'IconBack', run: () => { showPick(); return false; } }] : []), cancelBtn(),
    { label: isNew ? c('add') : c('save'), g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: () => { save(); return false; } }];
  stepFoot(props.win, foot);
}

function nameTyped() {
  if (!mountTouched) draft.mount = '/mnt/remotes/' + draft.name.trim().toLowerCase();
}

// The form as rclone takes it. An empty password leaves the saved one in place.
function input(): RemoteInput {
  const params: Record<string, string> = {};
  let path = '';
  for (const f of fields.value) {
    const v = draft[f.k]?.trim() ?? '';
    if (f.path) path = v;
    else params[f.k] = v;
  }
  if (token.value) params.token = token.value;
  return { name: draft.name.trim(), type: type.value!.id, path, mount: draft.mount.trim(), params };
}

async function test() {
  const m = missing();
  if (m) return fail(...m);
  tested.value = 'busy';
  const got = await admin.testRemote({ ...input(), ...(r ? { name: r.name } : {}) }).catch(() => null);
  tested.value = got?.ok ? 'ok' : 'no';
  if (!got?.ok) fail(c('remotes.testFailed'), at('#a-rtest'));
}

async function useToken() {
  if (!pasted.value.trim()) return fail(c('remotes.needToken'), at('#a-rtoken'));
  const got = await admin.remoteToken(type.value!.id, pasted.value.trim()).catch(() => null);
  if (!got?.token) return fail(c('remotes.tokenRejected'), at('#a-rtoken'));
  token.value = got.token;
  signing.value = false;
  pasted.value = '';
  toast(c('remotes.signedIn', { who: type.value!.oauth! }));
}

async function save() {
  const name = draft.name.trim(), mount = draft.mount.trim();
  if (!/^[a-z0-9][a-z0-9_-]*$/.test(name)) return fail(name ? c('lowercaseOnly') : c('remotes.needName'), at('#a-rname'));
  if (props.all.some((x) => x !== r && x.name === name)) return fail(c('nameTaken'), at('#a-rname'));
  if (!/^\/mnt\/[^/]+\/.+/.test(mount)) return fail(c('remotes.underMnt', { name }), at('#a-rmount'));
  const m = missing();
  if (m) return fail(...m);
  const done = await attempt(r ? admin.updateRemote(r.name, input()) : admin.addRemote(input()), at('#a-rname'));
  if (!done) return;
  props.win.close();
  toast(done.value.error ? c('remotes.savedNotMounted', { name }) : isNew ? c('remotes.addedAt', { name, mount }) : c('saved'), done.value.error ? 'warn' : 'ok');
  props.saved();
}

let stopKeys = () => {};
onMounted(() => {
  if (type.value) { defaults(type.value); showForm(); }
  else showPick();
  // Escape in the type search empties it first and closes the window only when it is empty.
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
  <template v-if="!type">
      <div class="a-pickhead"><TextField id="a-rq" v-model="query" :placeholder="c('remotes.searchType')" :aria-label="c('remotes.searchType')" width="medium" /></div>
      <div id="a-tiles" class="a-tiles">
        <button v-for="t in shown" :key="t.id" class="a-tile" :data-type="t.id" @click="choose(t)"><span class="tile"><G :n="t.g" /></span><span><b>{{ t.label }}</b><small>{{ t.sub }}</small></span></button>
        <div v-if="!shown.length" class="empty"><G n="IconSearch" /><div>{{ c('remotes.noType') }}</div></div>
      </div>
    </template>
    <template v-else>
      <div class="well glim-well">
        <div class="eyebrow">{{ type.label }}</div>
        <FieldRow v-if="type.oauth" :label="c('remotes.account')" :hint="c('remotes.accountTip')">
          <span v-if="signedIn" class="a-ctl"><Pill tone="ok">{{ c('remotes.signedInPill') }}</Pill><Button id="a-oauth" :label="c('remotes.otherAccount')" label-key="IconSignIn" @click="signing = true" /></span>
          <Button v-else id="a-oauth" :label="c('remotes.signInAt', { who: type.oauth })" label-key="IconSignIn" @click="signing = true" />
        </FieldRow>
        <template v-if="type.oauth && signing">
          <FieldRow :label="c('remotes.onYourPc')" :hint="c('remotes.onYourPcTip')"><CopyField class="glim-field--grow" :value="`rclone authorize &quot;${type.id}&quot;`" :label="c('remotes.onYourPc')" :text="copyWords()" /></FieldRow>
          <FieldRow :label="c('remotes.token')"><span class="a-ctl grow"><TextField id="a-rtoken" v-model="pasted" :placeholder="c('remotes.tokenPh')" spellcheck="false" autocomplete="off" width="wide" mono @keydown.enter="useToken" /><Button :label="c('remotes.takeToken')" label-key="IconCheck" @click="useToken" /></span></FieldRow>
        </template>
        <FieldRow v-for="f in fields" :key="f.k" :label="f.l" :hint="f.tip">
          <template #label>{{ f.l }}<template v-if="f.opt">{{ ' ' }}<small>{{ c('optional') }}</small></template></template>
          <Select v-if="f.dd" v-model="draft[f.k]" :data-rf="f.k" :options="selectOptions(f.dd)" rows="menu" />
          <PasswordField v-else-if="f.pw" v-model="draft[f.k]" width="medium" :data-rf="f.k" :placeholder="isNew ? '' : c('remotes.unchanged')" autocomplete="new-password" :text="eyeWords()" />
          <TextField v-else v-model="draft[f.k]" :width="WIDTHS[(f.w ?? 'm') as keyof typeof WIDTHS]" :data-rf="f.k" :placeholder="f.ph" />
        </FieldRow>
      </div>
      <div class="well glim-well">
        <FieldRow :label="c('remotes.name')" :hint="c('remotes.nameTip')"><TextField id="a-rname" v-model="draft.name" :placeholder="c('remotes.namePh')" width="medium" @input="nameTyped" /></FieldRow>
        <FieldRow :label="c('remotes.mountAt')" :hint="c('remotes.mountAtTip')"><PathField v-model="draft.mount" v-bind="pathField('a-rmount')" @input="mountTouched = true" /></FieldRow>
        <FieldRow :label="c('remotes.connection')">
          <Button id="a-rtest" :class="{ ok: tested === 'ok' }" :label="tested === 'ok' ? c('remotes.connected') : tested === 'busy' ? c('checking') : c('test')" :label-key="tested === 'ok' ? 'IconCheck' : tested === 'busy' ? 'IconRefresh' : 'IconHealth'" :disabled="tested === 'busy'" @click="test" />
        </FieldRow>
      </div>
    </template>
</template>
