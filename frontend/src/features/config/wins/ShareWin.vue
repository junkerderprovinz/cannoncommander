<script setup lang="ts">
import { Select } from '@glimstone/vue/Select';
import { Button, CopyField, FieldRow, FieldRowGroup, PasswordField, Pill, SubCard, TextArea, Toggle } from '@glimstone/vue';
import { nextTick, onMounted, ref } from 'vue';
import { admin, type Share } from '@/api/admin';
import { DAY, fmtDate, now } from '@/core/format';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { attempt, c, cn, fail, stepFoot } from '@/features/config/shared';
import { MESSAGE_MAX } from '@/features/share/link';
import { copyWords, eyeWords, selectOptions } from '@/ui/kit';

const props = defineProps<{ win: WinEntry; share: Share; saved: () => void }>();
const s = props.share;
const pwOn = ref(s.password), pwNew = ref(false), pw = ref('');
const expiry = ref('keep'), message = ref(s.message);
const pwInput = () => document.querySelector<HTMLInputElement>('#a-sharepw');
const keepLabel = () => c('shares.keep') + ', ' + (s.expires ? fmtDate(s.expires) : c('shares.never'));
const options = (): [string, string][] => [['keep', keepLabel()], ['1', cn(1, 'shares.fromNow')], ['7', cn(7, 'shares.fromNow')], ['30', cn(30, 'shares.fromNow')], ['never', c('shares.never_')]];

async function setNew() {
  pwNew.value = true;
  await nextTick();
  pwInput()?.focus();
}

async function save() {
  const asking = pwOn.value && (!s.password || pwNew.value);
  if (asking && pw.value.length < 6) return fail(c('shares.pwShort'), pwInput());
  const change: { expires?: number | null; password?: string; message?: string } = {};
  if (message.value.trim() !== s.message) change.message = message.value.trim();
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
  <SubCard>
    <FieldRowGroup>
      <FieldRow :label="c('shares.link')"><CopyField width="grow" :value="s.url" :label="c('shares.link')" :text="copyWords()" /></FieldRow>
      <FieldRow :label="c('shares.path')"><Pill v-for="p in s.paths" :key="p" class="mono">{{ p }}</Pill></FieldRow>
      <FieldRow :label="c('shares.created')"><Pill>{{ c('shares.createdBy', { date: fmtDate(s.created), by: s.owner }) }}</Pill></FieldRow>
      <FieldRow :label="c('shares.viewsLabel')"><Pill class="glim-num">{{ s.views }}</Pill></FieldRow>
    </FieldRowGroup>
  </SubCard>
  <SubCard>
    <FieldRow :label="c('shares.message')" :hint="c('shares.messageTip')"><TextArea id="a-shmsg" v-model="message" :maxlength="MESSAGE_MAX" rows="2" :placeholder="c('shares.messagePlaceholder')" width="wide" /></FieldRow>
    <FieldRow :label="c('shares.expires')"><Select v-model="expiry" :options="selectOptions(options())" /></FieldRow>
    <div id="a-shpwbox">
      <FieldRow :label="c('shares.askPw')" :hint="c('shares.askPwTip')"><Toggle v-model="pwOn" /></FieldRow>
      <template v-if="pwOn">
        <FieldRow v-if="s.password && !pwNew" :label="c('users.password')"><Pill>{{ c('users.pwSet') }}</Pill><Button :label="c('users.pwNew')" label-key="IconKey" @click="setNew" /></FieldRow>
        <FieldRow v-else :label="c('users.password')" :hint="c('shares.pwTip')"><PasswordField id="a-sharepw" v-model="pw" width="medium" autocomplete="new-password" :text="eyeWords()" /></FieldRow>
      </template>
    </div>
  </SubCard>
</template>
