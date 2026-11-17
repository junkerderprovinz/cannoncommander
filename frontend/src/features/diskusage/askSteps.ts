// Asks before the storeroom does anything, with one sentence for each step of the server's plan.
import { ask } from '@/features/transfer/ask';
import ConfirmBody from '@/ui/ConfirmBody.vue';

export async function askSteps(title: string, text: string[], verb: string, g: string) {
  return (await ask<boolean>({ title, body: ConfirmBody, props: { text } }, [{ label: verb, g, primary: true, answer: true }])) === true;
}
