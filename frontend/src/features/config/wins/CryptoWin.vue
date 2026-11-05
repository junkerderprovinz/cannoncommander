<script setup lang="ts">
import { FieldRow, Selector } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { closeBtn, type WinEntry } from '@/core/windows';
import { c, stepFoot } from '@/features/config/shared';
import { copyText, segItems } from '@/ui/kit';

const props = defineProps<{ win: WinEntry }>();
const COINS: [string, string, string][] = [['btc', 'Bitcoin', 'bc1q078lt57t4n5zq5md3knz3ythum0w78zmjw5eda'], ['eth', 'Ethereum', '0xFF6726C5bd76C8FD6b6bE7Ea5CEd4621fde5e841'],
  ['sol', 'Solana', 'GrTyhSbZVArdaZAr3TqWDrkEGahomLtNZJ41qPLm3dHd'], ['xrp', 'XRP', 'rwMK2nXqChT4JYWVypMypnpctDcM9jgWmG']];
const coin = ref('btc');
const addr = computed(() => COINS.find((x) => x[0] === coin.value)![2]);

onMounted(() => stepFoot(props.win, [closeBtn(), { label: c('info.copyAddress'), g: 'IconCopy', primary: true, run: () => { copyText(addr.value); return false; } }]));
</script>

<template>
  <div class="well glim-well">
    <FieldRow :label="c('info.coin')"><Selector id="cr-coin" v-model="coin" :items="segItems(COINS.map(([id, name]) => [id, name]))" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <div id="cr-addr" class="a-addr mono">{{ addr }}</div>
    <!-- XRP is the one coin here whose address would otherwise make a donor look for a tag. -->
    <p id="cr-note" class="a-text muted">{{ coin === 'xrp' ? c('info.xrpNote') : '' }}</p>
  </div>
</template>
