// The trash page's selection and order, kept while the page is left for a moment, as the prototype keeps them.
import { reactive } from 'vue';

export type SortKey = 'name' | 'from' | 'size' | 'deleted';
export const pick = reactive({ sel: new Set<string>(), sort: 'deleted' as SortKey, dir: -1, anchor: -1 });
