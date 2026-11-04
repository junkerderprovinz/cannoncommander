<script setup lang="ts">
// Stand-in for GlimStone's menu rows: a switch shows a checkbox where its glyph would stand, a choice among several
// keeps its glyph and ticks the current one at the end.
import { labelOf } from './actions';
import { G } from './glyphs';
import { isHead, itemOff, type MenuHead, type MenuItem, type Resolved } from './menu';

defineProps<{ list: Resolved[]; openK?: number }>();

const headText = (it: MenuHead) => (typeof it.head === 'function' ? it.head() : it.head);
const on = (it: MenuItem) => !!it.checked?.();
const role = (it: MenuItem) => it.checked
  ? { role: it.radio ? 'menuitemradio' : 'menuitemcheckbox', 'aria-checked': on(it) }
  : it.sub ? { role: 'menuitem', 'aria-haspopup': 'menu' as const, 'data-sub': '' } : { role: 'menuitem' };
const item = (it: Resolved) => it as MenuItem;
</script>

<template>
  <template v-for="(it, k) in list" :key="k">
    <div v-if="it === '-'" class="sep"></div>
    <div v-else-if="isHead(it)" class="head">{{ headText(it) }}<button v-if="it.run" class="hedit" :data-k="k" :data-tip="it.tip" :aria-label="it.tip"><G :n="it.g!" /></button></div>
    <button v-else :data-k="k" :class="{ on: item(it).radio && on(item(it)), open: k === openK }" :disabled="itemOff(item(it))" v-bind="role(item(it))">
      <span v-if="item(it).checked && !item(it).radio" class="cbox" :class="{ on: on(item(it)) }" role="presentation"><G n="IconCheck" /></span>
      <i v-else-if="item(it).color" class="cdot big" :style="{ '--c': item(it).color }"></i>
      <G v-else-if="item(it).g" :n="item(it).g!" />
      <span v-else class="ph"></span>
      <span>{{ labelOf(item(it)) }}</span>
      <span v-if="item(it).sub" class="chev"><G n="IconExpand" /></span>
      <span v-else-if="item(it).radio && on(item(it))" class="tick"><G n="IconCheck" /></span>
      <span v-else-if="item(it).key" class="kbd">{{ item(it).key }}</span>
      <span v-else-if="item(it).count != null" class="cnt glim-num">{{ item(it).count }}</span>
    </button>
  </template>
</template>
