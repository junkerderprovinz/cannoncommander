<script setup lang="ts">
// Nothing from a payment service loads before somebody opens this window and goes on to PayPal's own donation page,
// which takes cards without an account too.
import { onMounted, ref } from 'vue';
import { cancelBtn, type WinEntry } from '@/core/windows';
import Frow from '../kit/Frow.vue';
import { c, setFoot } from '../kit/kit';
import Seg from '../kit/Seg.vue';

const props = defineProps<{ win: WinEntry }>();
const often = ref('once'), amount = ref('25');
const PAYPAL = 'https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS';

onMounted(() => setFoot(props.win, [cancelBtn(), { label: c('info.toPaypal'), g: 'IconPayPal', primary: true, run: () => {
  window.open(`${PAYPAL}&amount=${amount.value}&currency_code=EUR${often.value === 'once' ? '' : '&recurring=1'}`, '_blank', 'noopener');
} }]));
</script>

<template>
  <div class="well">
    <Frow :label="c('info.often')"><Seg id="pp-often" v-model="often" :options="[['once', c('info.once')], ['month', c('info.monthly')], ['year', c('info.yearly')]]" /></Frow>
    <Frow :label="c('info.amount')"><Seg id="pp-amount" v-model="amount" :options="[['10', '10 €'], ['25', '25 €'], ['50', '50 €']]" /></Frow>
  </div>
  <div class="well a-text"><p>{{ c('info.paypalText') }}</p></div>
</template>
