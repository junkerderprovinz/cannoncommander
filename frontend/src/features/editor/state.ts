import { reactive } from 'vue';

// The markdown preview stays as it was left, from one file to the next.
export const opts = reactive({ split: true });
