<script setup lang="ts">
// F1: every action with its key, grouped as in the menu bar. A click on a key listens for the new combination.
import { segItems } from '@/ui/kit';
import { EmptyState, FieldRow, IconButton, Kbd, Selector, SubCard, TextField } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { type Action, actions, labelOf } from '@/core/actions';
import { G, glyphComponent } from '@/core/glyphs';
import { comboOf } from '@/core/keymap';
import type { MenuEntry } from '@/core/menu';
import { MENUS, menuName } from '@/core/menus';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import { keysFirst, stepFoot } from '@/features/config/shared';
import { assign, changed, check, combosOf, keyLabel, type KeyMap, mainCombo, MODIFIER_KEYS, preset, reset } from './keys';

const PRESETS = [['cc', 'CannonCommander'], ['tc', 'Total Commander']] as const;
const props = defineProps<{ win: WinEntry; current: KeyMap; defaults: KeyMap; declared: Record<string, string | undefined>; which: 'cc' | 'tc'; apply: (map: KeyMap, which: 'cc' | 'tc') => void }>();
const map = ref<KeyMap>({ ...props.current });
const which = ref(props.which);
const rec = ref<string | null>(null);
const pending = ref<{ combo?: string; msg: string } | null>(null);
const query = ref('');
const FIXED = ['tab', 'enter', 'insert', 'space', 'f10', 'esc'];

const name = (a: Action) => labelOf(a);
function groups(): [string, string[]][] {
  const seen = new Set<string>(), out: [string, string[]][] = [];
  const ids = (items: MenuEntry[]): string[] => items.flatMap((it) => (typeof it === 'string' ? [it] : typeof it === 'object' && 'sub' in it && it.sub ? ids(it.sub) : []))
    .filter((id) => actions[id] && !seen.has(id) && !!seen.add(id));
  MENUS.forEach(([, items], i) => out.push([menuName(i), ids(items)]));
  out.push([t('keys.more'), Object.keys(actions).filter((id) => !seen.has(id) && name(actions[id]!) && (combosOf(props.defaults, id).length || props.declared[id]))]);
  return out.filter(([, list]) => list.length);
}
const shown = computed(() => {
  const q = query.value.trim().toLowerCase();
  return groups().map(([g, list]) => [g, list.filter((id) => !q || name(actions[id]!).toLowerCase().includes(q) || combosOf(map.value, id).some((c) => keyLabel(c).toLowerCase().includes(q)))] as const).filter(([, l]) => l.length);
});

const main = (id: string) => mainCombo(map.value, id, props.declared[id]);
const stop = () => { rec.value = null; pending.value = null; };
function listen(id: string) {
  rec.value = rec.value === id ? null : id;
  pending.value = null;
}

// While a row listens, every key belongs to it. Escape alone gives up.
function key(e: KeyboardEvent) {
  if (!rec.value) return;
  e.preventDefault();
  e.stopPropagation();
  if (MODIFIER_KEYS.includes(e.key)) return;
  if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.shiftKey) return stop();
  const combo = comboOf(e);
  const v = check(map.value, combo, rec.value, pending.value?.combo === combo);
  if (!v.ok) {
    pending.value = v.reason === 'fixed' ? { msg: t('keys.fixed', { key: keyLabel(combo) }) }
      : v.reason === 'letter' ? { msg: t('keys.letter') }
      : { combo, msg: t('keys.taken', { key: keyLabel(combo), action: name(actions[(v as { other: string }).other]!) }) };
    return;
  }
  map.value = assign(map.value, combo, rec.value);
  stop();
}

function pickPreset(v: 'cc' | 'tc') {
  map.value = preset(v, props.defaults, (id) => !!actions[id]);
  stop();
}

let stopKeys = () => {};
onMounted(() => {
  stopKeys = keysFirst(key);
  stepFoot(props.win, [
    { start: t('keys.hint') },
    { label: t('keys.resetAll'), g: 'IconRefresh', run: () => { map.value = { ...props.defaults }; which.value = 'cc'; stop(); return false; } },
    cancelBtn(),
    { label: t('labels.take'), g: 'IconCheck', primary: true, run: () => props.apply(map.value, which.value) },
  ]);
});
onBeforeUnmount(() => stopKeys());
</script>

<template>
  <SubCard>
    <FieldRow :label="t('keys.preset')" :hint="t('keys.presetTip')"><Selector id="kb-preset" v-model="which" :items="segItems(PRESETS)" size="sm" activation="manual" inline button-height pairs-on-phone @update:model-value="pickPreset" /></FieldRow>
    <FieldRow :label="t('keys.search')"><TextField id="kb-q" v-model="query" :placeholder="t('keys.searchPh')" autocomplete="off" width="medium" /></FieldRow>
  </SubCard>
  <SubCard class="kb-well">
    <div id="kb-list" class="kb-list">
      <section v-for="[group, list] in shown" :key="group" class="kb-group">
        <div class="glim-eyebrow">{{ group }}</div>
        <div v-for="id in list" :key="id" class="kb-row" :class="{ rec: rec === id, own: changed(map, defaults, id) }" :data-kb="id">
          <span class="kb-name"><G v-if="actions[id]!.g" :n="actions[id]!.g!" /><span v-else class="ph"></span>{{ name(actions[id]!) }}</span>
          <button class="kb-key" :data-kb-rec="id" :aria-label="t('keys.change', { action: name(actions[id]!) })" @click="listen(id)">
            <span v-if="rec === id" class="muted">{{ t('keys.press') }}</span>
            <template v-else-if="main(id)"><Kbd>{{ keyLabel(main(id)!) }}</Kbd><span v-if="combosOf(map, id).length > 1" class="muted">+{{ combosOf(map, id).length - 1 }}</span></template>
            <span v-else class="muted">{{ t('keys.none') }}</span>
          </button>
          <span class="kb-acts">
            <IconButton v-if="main(id)" :tip="t('keys.clear')" :glyph="glyphComponent('IconClose')" :data-kb-clear="id" @click="map = Object.fromEntries(Object.entries(map).filter(([, a]) => a !== id)); stop()" />
            <IconButton v-if="changed(map, defaults, id)" :tip="t('keys.undo')" :glyph="glyphComponent('IconUndo')" :data-kb-reset="id" @click="map = reset(map, defaults, id); stop()" />
          </span>
          <div v-if="rec === id && pending" class="kb-warn">{{ pending.msg }}</div>
        </div>
      </section>
      <EmptyState v-if="!shown.length" nested :glyph="glyphComponent('IconSearch')" :title="t('keys.noMatch')" />
    </div>
    <div class="glim-eyebrow kb-fixedhead">{{ t('keys.fixedHead') }}</div>
    <div class="kbd-grid"><div v-for="k in FIXED" :key="k"><span>{{ t(`keys.fixedKeys.${k}.label`) }}</span><Kbd>{{ t(`keys.fixedKeys.${k}.key`) }}</Kbd></div></div>
  </SubCard>
</template>
