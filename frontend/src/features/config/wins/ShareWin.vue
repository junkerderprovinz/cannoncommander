<script setup lang="ts">
import { Select } from '@glimstone/vue/Select';
import { Button, CopyField, FieldRow, PasswordField, Pill, Toggle } from '@glimstone/vue';
import { nextTick, onMounted, ref } from 'vue';
import { admin, type Share } from '@/api/admin';
import { DAY, fmtDate, now } from '@/core/format';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { attempt, c, fail, stepFoot } from '@/features/config/shared';
import { copyWords, eyeWords, selectOptions } from '@/ui/kit';

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
    <FieldRow :label="c('shares.link')"><CopyField class="glim-field--grow" :value="s.url" :label="c('shares.link')" :text="copyWords()" /></FieldRow>
    <FieldRow :label="c('shares.path')"><Pill v-for="p in s.paths" :key="p" class="mono">{{ p }}</Pill></FieldRow>
    <FieldRow :label="c('shares.created')"><Pill>{{ c('shares.createdBy', { date: fmtDate(s.created), by: s.owner }) }}</Pill></FieldRow>
    <FieldRow :label="c('shares.viewsLabel')"><Pill class="glim-num">{{ s.views }}</Pill></FieldRow>
  </div>
  <div class="well glim-well">
    <FieldRow :label="c('shares.expires')"><Select v-model="expiry" :options="selectOptions(options())" rows="menu" /></FieldRow>
    <div id="a-shpwbox">
      <FieldRow :label="c('shares.askPw')" :hint="c('shares.askPwTip')"><Toggle v-model="pwOn" /></FieldRow>
      <template v-if="pwOn">
        <FieldRow v-if="s.password && !pwNew" :label="c('users.password')"><span class="a-ctl"><Pill>{{ c('users.pwSet') }}</Pill><Button :label="c('users.pwNew')" label-key="IconKey" @click="setNew" /></span></FieldRow>
        <FieldRow v-else :label="c('users.password')" :hint="c('shares.pwTip')"><PasswordField id="a-sharepw" v-model="pw" width="medium" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
      </template>
    </div>
  </div>
</template>
