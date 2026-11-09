<script setup lang="ts">
// The switch for sign-in through OAuth and the public address it runs under. Switching on without a stored address
// opens the field first, because the server takes the switch only with one.
import { Button, CopyField, FieldRow, TextField, Toggle } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { ApiError } from '@/api/client';
import { mcp, type McpOAuth } from '@/api/mcp';
import { toast } from '@/core/toast';
import { c, fail } from '@/features/config/shared';
import { copyWords } from '@/ui/kit';

const props = defineProps<{ oauth: McpOAuth; connector: string; changed: (oauth: McpOAuth) => void }>();
const draft = ref(props.oauth.issuer), opening = ref(false), busy = ref(false);
const field = ref<InstanceType<typeof TextField> | null>(null);

async function save(enabled: boolean, issuer: string) {
  busy.value = true;
  try {
    const got = await mcp.setOAuth(enabled, issuer.trim());
    opening.value = false;
    draft.value = got.issuer;
    props.changed(got);
    toast(c('mcp.oauthSaved'));
  } catch (e) {
    fail(e instanceof ApiError ? e.message : c('notSaved'), field.value?.input);
  } finally {
    busy.value = false;
  }
}

const on = computed({
  get: () => props.oauth.enabled || opening.value,
  set: (v: boolean) => {
    if (!v) {
      opening.value = false;
      if (props.oauth.enabled) void save(false, props.oauth.issuer);
    } else if (props.oauth.issuer) void save(true, props.oauth.issuer);
    else opening.value = true;
  },
});
</script>

<template>
  <FieldRow :label="c('mcp.oauth')" :hint="c('mcp.oauthTip')">
    <Toggle v-model="on" :disabled="busy" :refused="oauth.possible || oauth.enabled ? undefined : c('mcp.oauthNeedsSignIn')" data-mcp="oauth" />
  </FieldRow>
  <FieldRow v-if="on" :label="c('mcp.issuer')" :hint="c('mcp.issuerTip')">
    <TextField id="mcp-issuer" ref="field" v-model="draft" placeholder="https://files.example.com" width="wide" mono @keydown.enter="save(true, draft)" />
    <Button :label="c('save')" label-key="IconSave" :disabled="busy || !draft.trim() || (oauth.enabled && draft.trim() === oauth.issuer)" @click="save(true, draft)" />
  </FieldRow>
  <FieldRow v-if="oauth.active && connector" :label="c('mcp.connector')" :hint="c('mcp.connectorTip')">
    <CopyField width="grow" :value="connector" :label="c('mcp.connector')" :text="copyWords()" />
  </FieldRow>
</template>
