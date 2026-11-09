<script setup lang="ts">
// One client set up in three steps: a key, the configuration for that client, and the wait for its first call, which
// shows as a new last use while the window asks for the keys every few seconds. A client that signs in through OAuth
// gets the sign-in as its first step instead, and the window waits for its grant to appear.
import { Button, CopyField, EmptyState, FieldRow, InfoBubble, PasswordField, Pill, Selector, SubCard, TextField, Toggle } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ApiError } from '@/api/client';
import { mcp, type McpKeys, type McpOAuth, type McpSecret } from '@/api/mcp';
import { glyphComponent } from '@/core/glyphs';
import { closeBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';
import { copyText, copyWords, eyeWords, segItems } from '@/ui/kit';
import { staleSignIn } from '../proof';
import { signInFor } from './again';
import ClientMark from './ClientMark.vue';
import { type ConfigPaths, type McpClient, OTHER_CLIENT, snippetFor } from './clients';
import OAuthRows from './OAuthRows.vue';
import { KEY_PLACEHOLDER, KEY_VARIABLE } from './snippets';

const POLL = 3000;
const OS: [keyof ConfigPaths, string][] = [['windows', 'Windows'], ['macos', 'macOS'], ['linux', 'Linux'], ['unix', 'macOS, Linux']];

const props = defineProps<{
  win: WinEntry; client: McpClient; state: () => McpKeys | null; reload: () => Promise<void>;
  password: boolean; admin: boolean; endpoint: { origin: string; endpointPath: string }; connector: string;
  made: (got: McpSecret) => void; oauthChanged: (oauth: McpOAuth) => void;
}>();

const other = props.client.id === OTHER_CLIENT.id;
const app = other ? c('mcp.other') : props.client.name;
const data = computed(() => props.state());
const keys = computed(() => data.value?.keys ?? []);
const own = computed(() => keys.value.filter((k) => !k.viaOAuth));
const usable = computed(() => own.value.filter((k) => !k.unusable));
const full = computed(() => !!data.value && own.value.length >= data.value.limit);
const canMake = computed(() => data.value?.hostAllowsKeys !== false);

const mode = ref<'new' | 'pick'>('new');
const label = ref(other ? '' : props.client.name), write = ref(false), current = ref('');
const created = ref<{ key: string; id: string } | null>(null), picked = ref<{ id: string; since: number } | null>(null);
const stale = ref(false);
const nameField = ref<InstanceType<typeof TextField> | null>(null);
const at = (sel: string) => props.win.el?.querySelector(sel) ?? null;
const host = location.hostname;

// Each key's last use as the first list after opening has it. The list the card had may be minutes old, and a call
// from before the window must not count as the client's first.
const baseline = ref<Record<string, number> | null>(null);
const chosen = computed(() => keys.value.find((k) => k.id === (created.value?.id ?? picked.value?.id)));
const since = computed(() => (created.value ? 0 : picked.value ? Math.max(picked.value.since, baseline.value?.[picked.value.id] ?? Infinity) : Infinity));
const signedIn = computed(() => (props.client.oauth && baseline.value ? keys.value.find((k) => k.viaOAuth && k.client === props.client.id && !(k.id in baseline.value!)) : undefined));
const connected = computed(() => (props.client.oauth ? !!signedIn.value : !!chosen.value && chosen.value.lastUsedAt > since.value));
const time = (ms: number) => new Date(ms).toLocaleTimeString();

async function make() {
  const name = label.value.trim();
  if (!name) return fail(c('mcp.needName'), nameField.value?.input);
  if (props.password && !current.value) return fail(c('security.needCurrent'), at('#mcp-cpw'));
  let got: McpSecret;
  try {
    got = await mcp.create(name, other ? '' : props.client.id, write.value, props.password ? current.value : undefined);
  } catch (e) {
    if (staleSignIn(e)) return showStale();
    return fail(e instanceof ApiError ? e.message : c('notSaved'), props.password ? at('#mcp-cpw') : nameField.value?.input);
  }
  created.value = { key: got.key, id: got.item.id };
  props.made(got);
  // The key shows only here, so a stray Escape must not take it away.
  props.win.locked = true;
  stepFoot(props.win, [{ label: c('done'), g: 'IconCheck', primary: true, run: () => { props.win.locked = false; } }]);
}

function showStale() {
  stale.value = true;
  stepFoot(props.win, [closeBtn(), { label: c('security.signInAgain'), g: 'IconSignIn', primary: true, run: () => { signInFor({ do: 'create', client: props.client.id }); return false; } }]);
}

const snippet = computed(() => snippetFor(props.client, { ...props.endpoint, key: created.value?.key ?? KEY_PLACEHOLDER }));
const copyLabel = computed(() => c(props.client.setup.kind === 'command' ? 'mcp.copyCommand' : props.client.setup.kind === 'file' ? 'mcp.copyConfig' : 'mcp.copySettings'));
const intro = computed(() => {
  const s = props.client.setup;
  if (s.kind === 'form') return c('mcp.setup.form', { app, ui: s.ui });
  return c(`mcp.setup.${s.kind}`);
});
const paths = computed(() => {
  const s = props.client.setup;
  return s.kind === 'file' ? OS.flatMap(([os, name]) => (s.paths[os] ? [[name, s.paths[os]!] as const] : [])) : [];
});
const keyLine = computed(() => {
  const k = props.client.key;
  if (k.kind === 'env') return c('mcp.key.env', { app, var: KEY_VARIABLE }) + (k.unsure ? ' ' + c('mcp.key.envUnsure', { app }) : '');
  if (k.kind === 'dotenv') return c('mcp.key.dotenv', { app, var: KEY_VARIABLE, file: k.file });
  return c(`mcp.key.${k.kind}`, { app });
});
const keyTip = computed(() => {
  const k = props.client.key;
  if (k.kind === 'env') return c('mcp.key.envTip', { app, var: KEY_VARIABLE });
  if (k.kind === 'keyFile' && props.client.setup.kind === 'file') return c('mcp.key.keyFileTip', { app });
  return '';
});

let timer: ReturnType<typeof setInterval> | undefined;
onMounted(async () => {
  stepFoot(props.win, [closeBtn()]);
  if (!canMake.value || full.value) mode.value = usable.value.length ? 'pick' : 'new';
  await props.reload().catch(() => {});
  baseline.value = Object.fromEntries(keys.value.map((k) => [k.id, k.lastUsedAt]));
  timer = setInterval(() => { void props.reload().catch(() => {}); }, POLL);
});
onBeforeUnmount(() => clearInterval(timer));
</script>

<template>
  <SubCard v-if="client.group === 'cloud'" class="a-text"><p>{{ c('mcp.cloudTip') }}</p></SubCard>
  <div class="mcp-who">
    <span class="mcp-row-mark big"><ClientMark :client="client" /></span>
    <span><b>{{ app }}</b><small v-if="client.kind">{{ c(`mcp.kind.${client.kind}`) }}</small></span>
  </div>
  <SubCard v-if="client.note" class="a-text"><p>{{ c(`mcp.note.${client.note}`) }}</p></SubCard>

  <template v-if="client.oauth">
    <SubCard :title="c('mcp.oauth')">
      <div class="mcp-step">
        <OAuthRows v-if="admin && data" :oauth="data.oauth" :connector="connector" :changed="oauthChanged" />
        <p v-else-if="!data?.oauth.active" class="a-text">{{ c('mcp.setup.oauthOff') }}</p>
        <Pill v-else tone="ok">{{ c('security.on') }}</Pill>
      </div>
    </SubCard>
    <SubCard :title="c('mcp.stepConfig', { name: app })">
      <div class="mcp-step a-text">
        <p>{{ c(`mcp.setup.${client.oauth}`) }}</p>
        <template v-if="data?.oauth.active && connector">
          <pre class="mcp-snippet mono">{{ connector }}</pre>
          <Button :label="c('mcp.copyConnector')" label-key="IconCopy" @click="copyText(connector)" />
        </template>
        <p class="mcp-muted">{{ c('mcp.setup.proxy') }}</p>
      </div>
    </SubCard>
    <SubCard :title="connected ? c('mcp.stepDone') : c('mcp.stepSignIn')">
      <p v-if="signedIn" class="mcp-line" role="status"><span class="mcp-dot ok"></span>{{ c('mcp.signedIn', { name: signedIn.label, time: time(signedIn.createdAt) }) }}</p>
      <template v-else>
        <p class="mcp-line" role="status"><span class="mcp-dot"></span>{{ c('mcp.signInWait', { app }) }}</p>
        <p class="mcp-muted">{{ c('mcp.signInHint', { app }) }}</p>
      </template>
    </SubCard>
  </template>

  <template v-else>
    <SubCard :title="c('mcp.stepKey')">
      <template v-if="created">
        <p class="a-text">{{ c('mcp.copyNow') }}</p>
        <FieldRow :label="label.trim()"><CopyField width="grow" :value="created.key" :label="label.trim()" :text="copyWords()" /></FieldRow>
      </template>
      <EmptyState v-else-if="stale" nested :glyph="glyphComponent('IconSignIn')" :title="c('security.staleTitle')" :hint="c('mcp.stale')" />
      <template v-else>
        <FieldRow v-if="usable.length && canMake" :label="c('mcp.stepKey')">
          <Selector v-model="mode" :items="segItems([['new', c('mcp.newKey')], ['pick', c('mcp.existingKey')]])" size="sm" activation="manual" inline button-height pairs-on-phone />
        </FieldRow>
        <template v-if="mode === 'new'">
          <p v-if="!canMake" class="a-text">{{ c('mcp.hostRefused', { host }) }}</p>
          <p v-else-if="full" class="a-text">{{ c('mcp.full', { n: data!.limit }) }}</p>
          <template v-else>
            <FieldRow :label="c('sources.name')"><TextField id="mcp-label" ref="nameField" v-model="label" :placeholder="c('mcp.namePh')" width="medium" maxlength="64" @keydown.enter="make" /></FieldRow>
            <FieldRow :label="c('mcp.write')" :hint="c('mcp.writeTip', { perCall: data?.writeLimits.entriesPerCall ?? 0, perHour: data?.writeLimits.entriesPerHour ?? 0 })"><Toggle v-model="write" data-mcp="newWrite" /></FieldRow>
            <FieldRow v-if="password" :label="c('security.current')" :hint="c('security.proofTip')"><PasswordField id="mcp-cpw" v-model="current" width="medium" autocomplete="current-password" :text="eyeWords()" @keydown.enter="make" /></FieldRow>
            <div class="a-foot"><Button id="mcp-make" :label="c('mcp.makeKey')" label-key="IconKey" tone="accent" @click="make" /></div>
          </template>
        </template>
        <div v-else class="mcp-step">
          <div class="mcp-keys">
            <button v-for="k in usable" :key="k.id" type="button" :aria-pressed="picked?.id === k.id" @click="picked = { id: k.id, since: k.lastUsedAt }">
              <span class="mcp-row-mark"><ClientMark :client="undefined" for-key /></span><span>{{ k.label }} <span class="mono mcp-muted">…{{ k.hint }}</span></span>
            </button>
          </div>
          <p class="mcp-muted">{{ c('mcp.pickHint') }}</p>
        </div>
      </template>
    </SubCard>

    <SubCard :title="other ? c('mcp.stepConfigOther') : c('mcp.stepConfig', { name: app })">
      <div class="mcp-step">
        <p class="a-text">{{ client.node ? `${intro} ${c('mcp.setup.node', { app })}` : intro }}</p>
        <template v-if="snippet">
          <pre class="mcp-snippet mono" data-mcp="snippet">{{ snippet }}</pre>
          <Button :label="copyLabel" label-key="IconCopy" @click="copyText(snippet)" />
        </template>
        <template v-if="client.setup.kind === 'file'">
          <span class="mcp-muted">{{ c('mcp.setup.where') }}</span>
          <dl class="mcp-paths"><template v-for="[name, path] in paths" :key="name"><dt>{{ name }}</dt><dd class="mono">{{ path }}</dd></template></dl>
          <p v-if="client.setup.ui" class="a-text">{{ c('mcp.setup.fileUi', { app, ui: client.setup.ui }) }}</p>
          <p v-if="client.setup.paste" class="a-text">{{ c('mcp.setup.paste', { app, ui: client.setup.paste }) }}</p>
        </template>
        <p class="mcp-line a-text"><span>{{ keyLine }}</span><InfoBubble v-if="keyTip" :tip="keyTip" /></p>
      </div>
    </SubCard>

    <SubCard :title="connected ? c('mcp.stepDone') : c('mcp.stepWait')">
      <p v-if="connected && chosen" class="mcp-line" role="status" data-mcp="connected"><span class="mcp-dot ok"></span>{{ c('mcp.firstCall', { name: chosen.label, time: time(chosen.lastUsedAt) }) }}</p>
      <template v-else>
        <p class="mcp-line" role="status"><span class="mcp-dot"></span>{{ other ? c('mcp.waitLineOther') : c('mcp.waitLine', { app }) }}</p>
        <p v-if="!other" class="mcp-muted">{{ c('mcp.waitHint', { app }) }}</p>
      </template>
    </SubCard>
  </template>
</template>
