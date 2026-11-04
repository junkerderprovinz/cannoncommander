<script setup lang="ts">
import { armToast, dropToast, holdToast, type Toast, toasts } from './toast';

function act(t: Toast) {
  t.act!.run();
  dropToast(t.id);
}
</script>

<template>
  <Teleport to="#toasts">
    <div v-for="t in toasts" :key="t.id" class="toast glim-card" :class="t.kind" @mouseenter="holdToast(t)" @mouseleave="armToast(t)">
      <span class="dot"></span><span>{{ t.text }}</span><button v-if="t.act" class="btn" @click="act(t)">{{ t.act.label }}</button>
    </div>
  </Teleport>
</template>
