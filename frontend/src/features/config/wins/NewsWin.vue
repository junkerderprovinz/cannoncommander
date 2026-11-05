<script setup lang="ts">
import { Button, SubCard } from '@glimstone/vue';
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
    <Button v-if="unseen()" id="a-newall" :label="c('news.allSeen', { n: unseen() })" label-key="IconCheck" @click="all" />
    <Button v-else id="a-newall" :label="c('news.showAll', { n: NEWS.length })" label-key="IconEye" @click="all" />
  </div>
  <SubCard class="a-release" :title="version">
    <p class="a-text">{{ c('news.intro') }}</p>
    <div class="a-newlist">
      <div v-for="n in NEWS" :key="n.id" class="a-newrow" :class="{ seen: seen().includes(n.id) }">
        <span class="a-dot" :class="{ seen: seen().includes(n.id) }"></span><span class="tile"><G :n="n.g" /></span><span class="t">{{ n.text() }}</span>
        <Button :label="c('news.go')" label-key="IconForward" :data-newgo="n.id" @click="visit(n)" />
      </div>
    </div>
  </SubCard>
</template>
