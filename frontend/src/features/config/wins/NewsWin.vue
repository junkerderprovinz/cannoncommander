<script setup lang="ts">
import { onMounted } from 'vue';
import { G } from '@/core/glyphs';
import { setPref } from '@/core/prefs';
import { closeBtn, type WinEntry } from '@/core/windows';
import { NEWS, type News, seen, unseen } from '../help';
import { c, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry; version: string }>();

const all = () => setPref('newsSeen', unseen() ? NEWS.map((n) => n.id) : []);
function visit(n: News) {
  setPref('newsSeen', [...new Set([...seen(), n.id])]);
  props.win.close();
  n.go();
}
onMounted(() => stepFoot(props.win, [closeBtn()]));
</script>

<template>
  <div class="a-newtop">
    <button v-if="unseen()" id="a-newall" class="btn" @click="all"><G n="IconCheck" />{{ c('news.allSeen', { n: unseen() }) }}</button>
    <button v-else id="a-newall" class="btn" @click="all"><G n="IconEye" />{{ c('news.showAll', { n: NEWS.length }) }}</button>
  </div>
  <div class="well glim-well a-release"><span class="notch">{{ version }}</span>
    <p class="a-text">{{ c('news.intro') }}</p>
    <div class="a-newlist">
      <div v-for="n in NEWS" :key="n.id" class="a-newrow" :class="{ seen: seen().includes(n.id) }">
        <span class="a-dot" :class="{ seen: seen().includes(n.id) }"></span><span class="tile"><G :n="n.g" /></span><span class="t">{{ n.text() }}</span>
        <button class="btn" :data-newgo="n.id" @click="visit(n)"><G n="IconForward" />{{ c('news.go') }}</button>
      </div>
    </div>
  </div>
</template>
