<script setup lang="ts">
// The tags of one file or several: the x on a tag takes it away, a grey one sets it, Enter adds a new one.
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { G } from '@/core/glyphs';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import { byName, type Labelled, useLabels } from '@/stores/labels';
import Frow from '../config/kit/Frow.vue';
import { setFoot } from '../config/kit/kit';

const props = defineProps<{ win: WinEntry; sel: Labelled[]; folder: string }>();
const labels = useLabels();
const all = labels.tags.map((x) => x.name);
const has = (tag: string) => props.sel.every((k) => (labels.of(k, props.folder).tags || []).includes(tag));
const picked = reactive(new Set<string>());
// The tags the selection already carries are ticked once its folders' labels have arrived.
const loaded = () => props.sel.every((k) => labels.inFolder(k.at || props.folder) !== null);
const stop = watch(loaded, (ok) => {
  if (!ok) return;
  all.filter(has).forEach((tag) => picked.add(tag));
  queueMicrotask(() => stop());
}, { immediate: true });
const fresh = ref('');
const others = computed(() => all.filter((x) => !picked.has(x)));
const chosen = computed(() => [...picked].sort(byName));

function add() {
  const v = fresh.value.trim().replace(/^#/, '');
  if (!v) return;
  picked.add(v);
  fresh.value = '';
}
onMounted(() => setFoot(props.win, [cancelBtn(), { label: t('labels.take'), g: 'IconCheck', primary: true, run: () => { labels.set(props.sel, props.folder, { tags: [...picked] }); } }]));
</script>

<template>
  <div class="well">
    <Frow :label="t('labels.tag')" :tip="t('labels.tagTip')"><input id="newtag" v-model="fresh" class="field m" :placeholder="t('labels.tagPh')" autocomplete="off" @keydown.enter.prevent="add"></Frow>
    <div id="tags" class="tagchips">
      <span v-for="tag in chosen" :key="'on' + tag" class="tagchip on">#{{ tag }}<button class="x" :data-untag="tag" :data-tip="t('labels.untag')" :aria-label="t('labels.removeName', { name: tag })" @click="picked.delete(tag)"><G n="IconClose" /></button></span>
      <button v-for="tag in others" :key="tag" class="tagchip" :data-tag="tag" :data-tip="t('labels.addTag')" @click="picked.add(tag)">#{{ tag }}</button>
    </div>
  </div>
</template>
