<script setup lang="ts">
// GlimStone's button for what its Button cannot hold: a key cap or a count beside the words, or an address to go to.
import { buttonView, takePageHue, type ButtonTone } from '@glimstone/core';
import { useLabelMode } from '@glimstone/vue';
import { computed } from 'vue';
import { G } from '@/core/glyphs';

const props = withDefaults(defineProps<{ label: string; g: string; tone?: ButtonTone; href?: string }>(), { tone: 'neutral', href: undefined });
const mode = useLabelMode('buttons');
const view = computed(() => buttonView({ mode: mode.value, label: props.label, hasGlyph: true, tone: props.tone, variant: 'default', disabled: false, keepLabel: false }));
const reach = (e: Event) => { if (view.value.pageHue) takePageHue(e.currentTarget as HTMLElement); };
</script>

<template>
  <component :is="href ? 'a' : 'button'" :type="href ? undefined : 'button'" :href="href" :class="view.className" :style="view.style" :data-tip="view.tip" @mouseenter="reach" @focus="reach">
    <slot name="before" />
    <span v-if="view.showGlyph" class="glim-btn-glyph"><G :n="g" /></span>
    <span :class="view.labelClass">{{ label }}</span>
    <slot name="after" />
  </component>
</template>
