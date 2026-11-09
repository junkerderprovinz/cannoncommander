<script setup lang="ts">
// Where Ubuntu keeps programs out of user namespaces, an AppArmor rule for this app lets its thumbnailers run in the
// sandbox again. The app takes the rule on when it starts again.
import { Button, FieldRow } from '@glimstone/vue';
import { ref } from 'vue';
import { toast } from '@/core/toast';
import { t } from '@/i18n';
import { restartApp, sandboxOffered, setThumbnails, shell, warn } from './shell';

const busy = ref(false);

async function flip(allow: boolean) {
  busy.value = true;
  try {
    await setThumbnails(allow);
    const restart = { label: t('shellmenu.restartNow'), run: restartApp };
    if (shell.thumbnails?.restart) toast(t(allow ? 'shellmenu.thumbsOnRestart' : 'shellmenu.thumbsOffRestart'), 'ok', restart);
    else if (allow && shell.thumbnails?.way !== 'sandbox') toast(t('shellmenu.thumbsStillOut'), 'warn');
    else toast(t(allow ? 'shellmenu.thumbsOn' : 'shellmenu.thumbsOff'));
  } catch (e) {
    warn(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <FieldRow v-if="sandboxOffered()" :label="t('shellmenu.thumbs')" :hint="t('shellmenu.thumbsTip')">
    <Button v-if="shell.thumbnails?.restart" id="a-thumbs-restart" :label="t('shellmenu.restartNow')" label-key="IconRefresh" tone="accent" @click="restartApp" />
    <Button v-if="shell.thumbnails?.allowed" id="a-thumbs" :label="t('shellmenu.thumbsOffButton')" label-key="IconShieldOff" :disabled="busy" @click="flip(false)" />
    <Button v-else id="a-thumbs" :label="t('shellmenu.thumbsOnButton')" label-key="IconShieldOn" :tone="shell.thumbnails?.restart ? undefined : 'accent'" :disabled="busy" @click="flip(true)" />
  </FieldRow>
</template>
