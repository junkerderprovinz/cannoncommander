<script setup lang="ts">
// Nothing from a payment service loads before somebody opens this window and goes on to PayPal's own donation page,
// which takes cards without an account too.
import { segItems } from '@/ui/kit';
import { FieldRow, Selector } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry }>();
const often = ref('once'), amount = ref('25');
const PAYPAL = 'https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS';

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('info.toPaypal'), g: 'IconPayPal', primary: true, run: () => {
  window.open(`${PAYPAL}&amount=${amount.value}&currency_code=EUR${often.value === 'once' ? '' : '&recurring=1'}`, '_blank', 'noopener');
} }]));
</script>

<template>
  <div class="well glim-well">
    <FieldRow :label="c('info.often')"><Selector id="pp-often" v-model="often" :items="segItems([['once', c('info.once')], ['month', c('info.monthly')], ['year', c('info.yearly')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow :label="c('info.amount')"><Selector id="pp-amount" v-model="amount" :items="segItems([['10', '10 €'], ['25', '25 €'], ['50', '50 €']])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
  </div>
  <div class="well glim-well a-text"><p>{{ c('info.paypalText') }}</p></div>
</template>
