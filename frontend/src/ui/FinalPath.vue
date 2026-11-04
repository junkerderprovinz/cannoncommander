<script setup lang="ts">
// Where a tool's result lands, cut at the front so the name stays in sight.
defineProps<{ path: string }>();
</script>

<template>
  <span class="bk-path mono bk-final" :title="path"><bdi>{{ path }}</bdi></span>
</template>
