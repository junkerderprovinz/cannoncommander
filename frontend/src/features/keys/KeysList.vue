<script setup lang="ts">
// Every action with its key, grouped as in the menu bar: as cards of the settings tile, as sections in the F1 window's list.
// A click on a key listens for the new one. A key another action has asks first whether to swap or to replace.
import { Button, Card, EmptyState, IconButton } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { type Action, actions, labelOf } from '@/core/actions';
import { inScope } from '@/core/keymap';
import { G, glyphComponent } from '@/core/glyphs';
import { host } from '@/core/host';
import type { MenuEntry } from '@/core/menu';
import { MENUS, menuName } from '@/core/menus';
import { t } from '@/i18n';
import { assign, changed, check, combosOf, heldBack, keyLabel, type KeyMap, mainCombo, reservedBy, reset, restore, swap } from './keys';
import KeyField from './KeyField.vue';
import { declaredKeys } from './state';

const props = defineProps<{ map: KeyMap; base: KeyMap; query: string; cards?: boolean }>();
const emit = defineEmits<{ 'update:map': [map: KeyMap] }>();
const rec = ref<string | null>(null);
// What a row says: why a key cannot be taken, or the choice a clash leaves, which a reset can ask for too.
const pending = ref<{ id: string; msg: string; combo?: string; other?: string; reset?: boolean } | null>(null);

const name = (a: Action) => labelOf(a);
function groups(): [string, string[]][] {
  const seen = new Set<string>(), out: [string, string[]][] = [];
  const ids = (items: MenuEntry[]): string[] => items.flatMap((it) => (typeof it === 'string' ? [it] : typeof it === 'object' && 'sub' in it && it.sub ? ids(it.sub) : []))
    .filter((id) => actions[id] && !seen.has(id) && !!seen.add(id));
  MENUS.forEach(([, items], i) => out.push([menuName(i), ids(items)]));
  // Actions only a row menu or the command search offers can take a key too, apart from those that need their caller.
  out.push([t('keys.more'), Object.keys(actions).filter((id) => !seen.has(id) && name(actions[id]!) && !actions[id]!.hidden && !actions[id]!.scope)]);
  // Each scope, such as the editor, gets a group of its own, as its keys only count there.
  const scoped = Object.values(actions).filter((a) => a.scope && name(a));
  for (const scope of new Set(scoped.map((a) => a.scope!))) out.push([t(`keys.scope.${scope}`), scoped.filter((a) => a.scope === scope).map((a) => a.id)]);
  return out.filter(([, list]) => list.length);
}
const shown = computed(() => {
  const q = props.query.trim().toLowerCase();
  const hit = (id: string) => !q || name(actions[id]!).toLowerCase().includes(q) || combosOf(props.map, id).some((c) => keyLabel(c).toLowerCase().includes(q));
  return groups().map(([g, list]) => [g, list.filter(hit)] as const).filter(([, l]) => l.length);
});

const usable = (combo: string) => !reservedBy(combo, host.desktop);
// The key a row shows is the one menus show, unless the browser keeps that one and the action has another that works.
function main(id: string) {
  const m = mainCombo(props.map, id, declaredKeys()[id]);
  return m && !usable(m) ? combosOf(props.map, id).find(usable) ?? m : m;
}
const blocked = (combo: string) => {
  const why = reservedBy(combo, host.desktop);
  return why ? t(`keys.${why}Here`) : undefined;
};
// Every key of a row, the one menus show first.
const chips = (id: string) => {
  const first = main(id);
  return [...(first ? [first] : []), ...combosOf(props.map, id).filter((c) => c !== first)].map((c) => ({ label: keyLabel(c), blocked: blocked(c) }));
};

const stop = () => { rec.value = null; pending.value = null; };
const set = (map: KeyMap) => { emit('update:map', map); stop(); };
function listen(id: string) {
  rec.value = rec.value === id ? null : id;
  pending.value = null;
}
function leave(e: FocusEvent, id: string) {
  if ((rec.value === id || pending.value?.id === id) && !(e.currentTarget as Element).contains(e.relatedTarget as Node | null)) stop();
}

function press(pressed: string) {
  const id = rec.value!, scope = actions[id]!.scope;
  const combo = scope ? inScope(scope, pressed) : pressed;
  const v = check(props.map, combo, id, host.desktop);
  if (v.ok) return set(assign(props.map, combo, id));
  const key = keyLabel(combo);
  if (v.reason !== 'taken') {
    pending.value = { id, msg: t(`keys.${v.reason}`, { key }) };
    return;
  }
  // Pressing the same key once more takes it without the buttons.
  if (pending.value?.combo === combo) return set(assign(props.map, combo, id));
  pending.value = clash(id, combo, v.other);
}
const clash = (id: string, combo: string, other: string, back?: boolean) => ({
  id, combo, other, reset: back,
  msg: t(combosOf(props.map, id).length ? 'keys.taken' : 'keys.takenAlone', { key: keyLabel(combo), action: name(actions[other]!) }),
});
function take(swapped: boolean) {
  const { id, combo, reset: back } = pending.value!;
  set(back ? restore(props.map, props.base, id, swapped) : (swapped ? swap : assign)(props.map, combo!, id));
}
// A reset whose key another action has taken asks first, so that action never ends up without a key unnoticed.
function undo(id: string) {
  const held = heldBack(props.map, props.base, id)[0];
  if (!held) return set(reset(props.map, props.base, id));
  rec.value = null;
  pending.value = clash(id, held, props.map[held]!, true);
}
const clear = (id: string) => set(Object.fromEntries(Object.entries(props.map).filter(([, a]) => a !== id)));
</script>

<template>
  <component :is="cards ? Card : 'section'" v-for="[group, list] in shown" :key="group" class="kb-group" v-bind="cards ? { title: group } : {}">
    <div v-if="!cards" class="glim-eyebrow">{{ group }}</div>
    <div v-for="id in list" :key="id" class="kb-row" :class="{ rec: rec === id, own: changed(map, base, id) }" :data-kb="id" @focusout="leave($event, id)">
      <span class="kb-name"><G v-if="actions[id]!.g" :n="actions[id]!.g!" /><span v-else class="ph"></span>{{ name(actions[id]!) }}</span>
      <KeyField
        :keys="chips(id)"
        :listening="rec === id" :listening-text="t('keys.press')" :empty-text="t('keys.none')"
        :data-kb-rec="id" :aria-label="t('keys.change', { action: name(actions[id]!) })"
        @click="listen(id)" @press="press" @cancel="stop" />
      <span class="kb-acts">
        <IconButton v-if="changed(map, base, id)" :tip="t('keys.reset')" :glyph="glyphComponent('IconResetArrow')" :data-kb-reset="id" @click="undo(id)" />
        <IconButton v-if="main(id)" :tip="t('keys.clear')" :glyph="glyphComponent('IconTrash')" :data-kb-clear="id" @click="clear(id)" />
      </span>
      <div v-if="pending?.id === id" class="kb-warn" role="alert">
        <span>{{ pending.msg }}</span>
        <span v-if="pending.other" class="kb-choice">
          <Button v-if="combosOf(map, id).length" id="kb-swap" :label="t('keys.swap')" label-key="IconSync" @click="take(true)" />
          <Button id="kb-replace" :label="t('keys.replace')" label-key="IconCheck" @click="take(false)" />
        </span>
      </div>
    </div>
  </component>
  <component :is="cards ? Card : 'div'" v-if="!shown.length"><EmptyState nested :glyph="glyphComponent('IconSearch')" :title="t('keys.noMatch')" /></component>
</template>
