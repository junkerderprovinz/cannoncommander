<script setup lang="ts">
// One rclone option of a remote as a field: a switch for a yes or no, a list where rclone names the choices, a password
// field for a secret and a line for the rest. A saved secret stays as it is while its field is left empty.
import { FieldRow, PasswordField, TextField, Toggle } from '@glimstone/vue';
import { Select } from '@glimstone/vue/Select';
import { computed } from 'vue';
import type { Provider, RemoteOption } from '@/api/admin';
import { c } from '@/features/config/shared';
import { eyeWords } from '@/ui/kit';
import { applies, optionHint, optionLabel } from '../remotes';

const props = defineProps<{ o: RemoteOption; values: Record<string, string>; provider: Provider | null; type: string; saved: boolean }>();
// rclone lists the vendors of WebDAV and the providers of S3 without making them exclusive, but a typo there only
// shows up as a strange error much later.
const PICKED = new Set(['vendor', 'provider']);
const WIDE = new Set(['url', 'endpoint', 'host_keys', 'remote', 'upstreams']);

const choices = computed(() => (props.o.examples ?? []).filter((e) => applies(e.provider, props.values.provider)).map((e) => ({ value: e.value, label: e.help || e.value })));
const placeholder = computed(() => props.provider?.defaults?.[props.o.name] || props.o.default || choices.value[0]?.value || '');
// A switch is off where rclone's default is off, and one left as rclone has it stays out of the parameters.
const on = computed({
  get: () => (props.values[props.o.name] || props.o.default) === 'true',
  set: (v: boolean) => { props.values[props.o.name] = String(v) === props.o.default ? '' : String(v); },
});
</script>

<template>
  <FieldRow :label="optionLabel(o.name)" :hint="optionHint(o, provider, type)">
    <Toggle v-if="o.type === 'bool'" v-model="on" :data-rf="o.name" />
    <Select v-else-if="(o.exclusive || PICKED.has(o.name)) && choices.length" v-model="values[o.name]" :options="choices" :trigger-attrs="{ 'data-rf': o.name }" :placeholder="placeholder" />
    <PasswordField v-else-if="o.secret" v-model="values[o.name]" width="medium" :data-rf="o.name" :placeholder="saved ? c('remotes.unchanged') : placeholder" autocomplete="new-password" :text="eyeWords()" />
    <TextField v-else v-model="values[o.name]" :width="WIDE.has(o.name) ? 'wide' : 'medium'" :data-rf="o.name" :placeholder="placeholder" spellcheck="false" mono />
  </FieldRow>
</template>
