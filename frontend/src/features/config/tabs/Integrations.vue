<script setup lang="ts">
// The office server is a choice: none uses the built-in previews, OnlyOffice and Euro-Office share one connection,
// Collabora speaks WOPI.
import { computed, onMounted, ref } from 'vue';
import { admin, type AppSettings, type OfficeKind, type System } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import Card from '../kit/Card.vue';
import CopyField from '../kit/CopyField.vue';
import Dropdown from '../kit/Dropdown.vue';
import Frow from '../kit/Frow.vue';
import { c, fail, shake } from '../kit/kit';
import PwField from '../kit/PwField.vue';
import Sw from '../kit/Sw.vue';

const app = ref<AppSettings | null>(null), sys = ref<System | null>(null);
const test = ref<'idle' | 'busy' | 'ok' | 'bad'>('idle');
const testBtn = ref<HTMLButtonElement | null>(null);
onMounted(() => {
  admin.app().then((a) => { app.value = a; }, () => {});
  admin.system().then((s) => { sys.value = s; }, () => {});
});

function save() {
  admin.saveApp(app.value!).then(() => toast(c('saved')), () => toast(c('notSaved'), 'warn'));
}
const flag = (k: 'thumbs' | 'videoPreview' | 'webdav') => computed({ get: () => !!app.value?.[k], set: (v: boolean) => { app.value![k] = v; save(); } });
const thumbs = flag('thumbs'), videoPreview = flag('videoPreview'), webdav = flag('webdav');
const kind = computed({ get: () => app.value?.office.kind ?? 'none', set: (k: OfficeKind) => { app.value!.office.kind = k; test.value = 'idle'; save(); } });
const office = computed(() => app.value?.office);
const needsSecret = computed(() => kind.value === 'onlyoffice' || kind.value === 'eurooffice');

async function testOffice() {
  const o = office.value!;
  if (!o.url.trim()) return fail(c('integrations.needUrl'), testBtn.value);
  if (!/^https?:\/\/[^/\s]+/.test(o.url.trim())) return fail(c('integrations.badUrl'), testBtn.value);
  test.value = 'busy';
  const got = await admin.testOffice({ ...o, url: o.url.trim() }).catch(() => ({ ok: false, reason: 'unreachable' as const }));
  test.value = got.ok ? 'ok' : 'bad';
  if (got.ok) return;
  shake(testBtn.value);
  toast(got.reason === 'secret' ? c('integrations.wantsSecret') : c('integrations.unreachable'), 'warn');
}
</script>

<template>
  <template v-if="app">
    <Card :title="c('integrations.office')">
      <Frow :label="c('integrations.officeKind')" :tip="c('integrations.officeKindTip')">
        <Dropdown v-model="kind" :options="[['none', c('integrations.none')], ['onlyoffice', 'OnlyOffice'], ['eurooffice', 'Euro-Office'], ['collabora', 'Collabora Online']]" />
      </Frow>
      <template v-if="office && kind !== 'none'">
        <Frow :label="c('integrations.url')" :tip="c('integrations.urlTip')">
          <span class="a-ctl"><input id="a-oourl" v-model="office.url" class="field w" placeholder="https://office.tower.lan" @input="test = 'idle'" @change="save">
            <button id="a-ootest" ref="testBtn" class="btn" :class="{ ok: test === 'ok', bad: test === 'bad' }" :disabled="test === 'busy'" @click="testOffice"><G :n="test === 'ok' ? 'IconCheck' : test === 'bad' ? 'IconClose' : test === 'busy' ? 'IconRefresh' : 'IconHealth'" />{{ test === 'ok' ? c('remotes.connected') : test === 'bad' ? c('integrations.rejected') : test === 'busy' ? c('checking') : c('test') }}</button></span>
        </Frow>
        <Frow v-if="needsSecret" :label="c('integrations.secret')" :tip="c('integrations.secretTip')"><PwField id="a-oosecret" v-model="office.secret" placeholder="JWT_SECRET" @change="save" /></Frow>
      </template>
    </Card>
    <Card :title="c('integrations.media')">
      <Frow :label="c('integrations.thumbs')" :tip="c('integrations.thumbsTip')"><Sw v-model="thumbs" /></Frow>
      <Frow :label="c('integrations.video')" :tip="c('integrations.videoTip')"><Sw v-model="videoPreview" /></Frow>
      <Frow label="ffmpeg" :tip="c('integrations.ffmpegTip')"><span class="pill mono">{{ sys ? `${sys.version.ffmpeg}, ${sys.paths.ffmpeg}` : '' }}</span></Frow>
    </Card>
    <Card :title="c('integrations.webdavRclone')">
      <Frow :label="c('integrations.webdav')" :tip="c('integrations.webdavTip')"><Sw v-model="webdav" /></Frow>
      <Frow v-if="webdav" :label="c('integrations.address')"><CopyField :value="app.webdavUrl" /></Frow>
      <Frow label="rclone" :tip="c('integrations.rcloneTip')"><span class="pill mono">{{ sys ? `${sys.version.rclone}, ${sys.paths.rclone}` : '' }}</span></Frow>
    </Card>
  </template>
</template>
