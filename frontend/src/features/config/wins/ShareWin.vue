<script setup lang="ts">
import { CopyField, PasswordField } from '@glimstone/vue';
import { nextTick, onMounted, ref } from 'vue';
import { admin, type Share } from '@/api/admin';
import { DAY, fmtDate, now } from '@/core/format';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Dropdown from '@/ui/Dropdown.vue';
import Frow from '@/ui/Frow.vue';
import { attempt, c, fail, stepFoot } from '@/features/config/shared';
import Sw from '@/ui/Sw.vue';
import { copyWords, eyeWords } from '@/ui/kit';

const props = defineProps<{ win: WinEntry; share: Share; saved: () => void }>();
const s = props.share;
const pwOn = ref(s.password), pwNew = ref(false), pw = ref('');
const expiry = ref('keep');
const pwInput = () => document.querySelector<HTMLInputElement>('#a-sharepw');
const keepLabel = () => c('shares.keep') + ', ' + (s.expires ? fmtDate(s.expires) : c('shares.never'));
const options = (): [string, string][] => [['keep', keepLabel()], ['1', c('shares.fromNow1')], ['7', c('shares.fromNow', { n: 7 })], ['30', c('shares.fromNow', { n: 30 })], ['never', c('shares.never_')]];

async function setNew() {
  pwNew.value = true;
  await nextTick();
  pwInput()?.focus();
}

async function save() {
  const asking = pwOn.value && (!s.password || pwNew.value);
  if (asking && pw.value.length < 6) return fail(c('shares.pwShort'), pwInput());
  const change: { expires?: number | null; password?: string } = {};
  if (expiry.value === 'never') change.expires = null;
  else if (expiry.value !== 'keep') change.expires = now() + Number(expiry.value) * DAY;
  if (!pwOn.value && s.password) change.password = '';
  if (asking) change.password = pw.value;
  if (!(await attempt(admin.updateShare(s.id, change), pwInput()))) return;
  props.win.close();
  toast(c('saved'));
  props.saved();
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('save'), g: 'IconSave', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div class="well glim-well">
    <Frow :label="c('shares.link')"><CopyField class="w" :value="s.url" :label="c('shares.link')" :text="copyWords()" /></Frow>
    <Frow :label="c('shares.path')"><span v-for="p in s.paths" :key="p" class="pill mono">{{ p }}</span></Frow>
    <Frow :label="c('shares.created')"><span class="pill">{{ c('shares.createdBy', { date: fmtDate(s.created), by: s.owner }) }}</span></Frow>
    <Frow :label="c('shares.viewsLabel')"><span class="pill glim-num">{{ s.views }}</span></Frow>
  </div>
  <div class="well glim-well">
    <Frow :label="c('shares.expires')"><Dropdown v-model="expiry" :options="options()" /></Frow>
    <div id="a-shpwbox">
      <Frow :label="c('shares.askPw')" :tip="c('shares.askPwTip')"><Sw v-model="pwOn" /></Frow>
      <template v-if="pwOn">
        <Frow v-if="s.password && !pwNew" :label="c('users.password')"><span class="a-ctl"><span class="pill">{{ c('users.pwSet') }}</span><button class="btn" @click="setNew"><G n="IconKey" />{{ c('users.pwNew') }}</button></span></Frow>
        <Frow v-else :label="c('users.password')" :tip="c('shares.pwTip')"><PasswordField id="a-sharepw" v-model="pw" class="m" autocomplete="new-password" :text="eyeWords()" /></Frow>
      </template>
    </div>
  </div>
</template>
